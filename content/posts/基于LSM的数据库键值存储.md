---
title: "基于LSM索引的数据库键值存储"
date: 2020-10-28T11:19:00+08:00
draft: false
tags: ["数据库", "索引", "键值数据库"]
---

数据库中使用索引可以加快查询的速度，索引的意思是说，给某些数据添加类似路标的记号，这样从索引中就可以直接检索到该数据的位置。以MySQL为例，添加主键时默认会为该属性加上主键索引，除此之外，MySQL中还有联合索引、唯一索引等。以MySQL等为代表的关系型数据中索引常常用B+树来实现，在B+树中，叶子节点包含了所有的关键字的信息，并且按照主键大小排列，非叶子节点中存放着指向叶子节点中的指针（页号和页对应列的最小记录）。除了用B+树实现索引外，常见的还有Hash索引、全文索引以及LSM树索引。最简单的是Hash索引，标注键在数据库的位置，直接通过hash映射找到键的位置即可。而LSM树常常用在键值数据库的索引实现上。

### LSM树

LSM（Log-Structured Merge Tree）树索引伴随着**键值数据库**而出现，以RocksDB、LevelDB为代表，比如国内著名的分布式数据库TiDB的底层存储实现就是用的RocksDB。存键值对最简单的方式是直接以追加写的方式写入一个文件即可，但是不能一直写呀，否则这个文件会变得很大，同时可能存在很多重复的值（比如对访问量的计数），所以要把文件分成多个段，这样一个文件写满之后进行压缩，剔除重复的、不要的数据，然后在新的文件中写入。因为磁盘的特性，顺序写入的性能很高，但是查找数据是个问题，每次都要全表扫描，如果磁盘中的数据是有序的就好了，查找就会很快（二分查找）。

![](https://shiniao.fun/images/20201028165410.png)

这种数据存储的方式其实叫做**SSTable**（排序字符串表），在每个SSTable文件中，数据按照键的顺序排序，当一个SSTable满了之后，通过合并压缩的方式，删除旧值以及重复的值。另外，数据肯定不会直接写入磁盘中的SSTable，首先会写入内存中，也叫做内存表，当内存表超出大小后才作为SSTable文件写入磁盘，然后在后台定期压缩。

那么如何保证写入内存表的键值对是有序的？可以使用红黑树、B+树来实现，也有使用**基数树**来实现的（比如下面介绍的bitcask就使用基数树来对键值对排序）。这种先在内存中构建一颗有序树，当大小超出后写入磁盘的方式就是**LSM树**。

![](https://shiniao.fun/images/20201028141704.png)

在读取数据的时候，首先在内存表中查找键所在的文件位置，然后在最近的SSTable中查找，没有的话继续找之前的。同时磁盘中的SSTable会定期合并压缩，成为新的SSTable，这样可以节省空间，提高性能。

![](https://shiniao.fun/images/20201028150112.png)

LSM树的主要优点是所有前台写入都发生在内存中，并且所有后台写入都保持顺序访问模式。有着很高的写入吞吐量。

上文说的LevelDB来源于Google的SSTable论文，而RocksDB是对levelDB的一些改进，为了更加深入的了解LSM以及SSTable，本文以类似的键值数据库[bitcask](https://github.com/prologic/bitcask)为例，看看它们具体是怎么实现的。

### bitcask

bitcask在保证键的顺序上使用了一种**自适应基数树**（ART）的算法结构，论文在这：

> https://db.in.tum.de/~leis/papers/ART.pdf

ART是对基数树的一种改进算法，基数树就是前缀（Trie）树，只不过更节省空间。在前缀树中，每个节点是一个单词，而基数树中，如果一个节点是父节点的唯一子节点的话，那么该子节点将会与父节点进行合并。以插入hello、hat、have三个单词为例：

```shell
# trie
		e - l - l - o
	  /
* - h - a - t
	      \
	       v - e

# radix
			*
           /
        (ello)
         /
* - h - * -(a) - * - (t) - *
                 \
                 (ve)
                   \
                    *
```

前缀算法需要十个节点，而基数树算法只需要五个节点就能表示。

在bitcask中，SSTable表示如下：

```go
type datafile struct {
	sync.RWMutex

	id           int
	r            *os.File
	ra           *mmap.ReaderAt
	w            *os.File
	offset       int64
	// decode and encode 二进制
	dec          *codec.Decoder
	enc          *codec.Encoder
	maxKeySize   uint32
	maxValueSize uint64
}
```

在查找数据的时候，首先会从基于ART实现的索引中找到键所在的位置，然后在当前SSTable和之前的分别查找。

```go

// Get retrieves the value of the given key. If the key is not found or an/I/O
// error occurs a null byte slice is returned along with the error.
func (b *Bitcask) Get(key []byte) ([]byte, error) {
	var df data.Datafile

	b.mu.RLock()
	// 优化，可以通过bloom 算法判断键存不存在，存在的话继续search
	// 不存在的话直接error
	// 从 ART 索引找到键的位置
	// key: [fileid, offset, size]
	value, found := b.trie.Search(key)
	if !found {
		b.mu.RUnlock()
		return nil, ErrKeyNotFound
	}

	item := value.(internal.Item)
	// 如果这个键在当前 SSTable中
	if item.FileID == b.curr.FileID() {
		// 查当前
		df = b.curr
	} else {
		// 查之前SSTable（磁盘中）
		df = b.datafiles[item.FileID]
	}
	// 读取
	e, err := df.ReadAt(item.Offset, item.Size)
	b.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	// 校验
	checksum := crc32.ChecksumIEEE(e.Value)
	if checksum != e.Checksum {
		return nil, ErrChecksumFailed
	}

	return e.Value, nil
}
```

写入键值对的时候，

1. 如果SSTable超出大小了，关闭当前SSTable，并再次打开，不过这次只能读了，不能写入（归档）
2. 新建一个SSTable，分配读写权限
3. 如果没有超出大小，写入即可（encode）

```go
// Put stores the key and value in the database.
func (b *Bitcask) Put(key, value []byte) error {
	......
	b.mu.Lock()
	// 写入
	offset, n, err := b.put(key, value)
	......
	item := internal.Item{FileID: b.curr.FileID(), Offset: offset, Size: n}
	// 加入ART索引
	b.trie.Insert(key, item)
	b.mu.Unlock()
	return nil
}

// put inserts a new (key, value). Both key and value are valid inputs.
func (b *Bitcask) put(key, value []byte) (int64, int64, error) {
	size := b.curr.Size()
	// 一个SSTable（默认1MB）装不下了
	if size >= int64(b.config.MaxDatafileSize) {
		// 关闭当前SSTable
		err := b.curr.Close()
		if err != nil {
			return -1, 0, err
		}

		id := b.curr.FileID()
		// 将当前SSTable归档，设为只读
		df, err := data.NewDatafile(b.path, id, true, b.config.MaxKeySize, b.config.MaxValueSize)
		if err != nil {
			return -1, 0, err
		}

		b.datafiles[id] = df

		id = b.curr.FileID() + 1
		// 新建SSTable文件（id+1），并分配读写权限
		curr, err := data.NewDatafile(b.path, id, false, b.config.MaxKeySize, b.config.MaxValueSize)
		if err != nil {
			return -1, 0, err
		}
		b.curr = curr
	}
	// 写入 key-value
	e := internal.NewEntry(key, value)
	return b.curr.Write(e)
}
```

而合并和压缩SSTable会在后台周期性的执行：

1. 创建临时表
2. 查找键是否在 ART 索引叶子节点上，将存在的放入临时表中（合并压缩，老的会被新的代替）
3. 移除所有SSTable
4. 重命名临时表为新的SSTable
5. 重新打开数据库

```go
	// Rewrite all key/value pairs into merged database
	// Doing this automatically strips deleted keys and
	// old key/value pairs
	err = b.Fold(func(key []byte) error {
		// b.Get的key都是经过ART处理过的
		value, err := b.Get(key)
		if err != nil {
			return err
		}

		if err := mdb.Put(key, value); err != nil {
			return err
		}

		return nil
	})


// Fold iterates over all keys in the database calling the function `f` for
// each key. If the function returns an error, no further keys are processed
// and the error returned.
func (b *Bitcask) Fold(f func(key []byte) error) (err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// key在ART的叶子节点，返回true，否则返回false，不处理
	b.trie.ForEach(func(node art.Node) bool {
		if err = f(node.Key()); err != nil {
			return false
		}
		return true
	})

	return
}
```



参考文献：

[1]  "The Adaptive Radix Tree:ARTful Indexing for Main-Memory Databases", Viktor Leis, Alfons Kemper, Thomas Neumann.

[2] 《设计数据密集型应用》

[3] https://github.com/prologic/bitcask

[4] https://stackoverflow.com/questions/14708134/what-is-the-difference-between-trie-and-radix-trie-data-structures