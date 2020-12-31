---
title: "键值数据库bitcask实现分析"
date: 2020-12-30T11:15:34+08:00
draft: false
tags: ['数据库', 'k-v store'] 
---

[bitcask](https://github.com/prologic/bitcask) 是一个用 go 写的高性能的键值存储数据库，类似 LevelDB 使用 LSM，但是更简单，独特之处在于在索引结构的实现上使用了称之为 ART 结构的树。希望借由 bitcask 的源码探究，管中窥豹，进而了解 LevelDB 等数据库的实现（其实是不懂 C++呀）。

整个数据库的运行由一个 bitcask 结构控制：

```go
type Bitcask struct {
	mu sync.RWMutex // 读写互斥锁

	*flock.Flock // 文件锁

	config    *config.Config        // 数据库配置
	options   []Option              // 数据库选项
	path      string                // 存储路径
	curr      data.Datafile         // 当前 datafile
	datafiles map[int]data.Datafile // 所有的 datafile
	trie      art.Tree              // trie 树
	indexer   index.Indexer         // 索引
	metadata  *metadata.MetaData    // 源数据
	isMerging bool                  // 是否正在合并
}
```

其中声明了索引、数据文件、配置项、源数据等属性。

bitcask 中定义的几个基本概念：

- Entry：entry 代表一个键值对，用来在磁盘字节序列和对象之间转换。
- Index：索引，索引使用了 ART 树结构，是 Trie 树的一个优化变种。
- Datafile：数据文件，写入磁盘的基本结构。

### 索引

bitcask 的索引使用了一篇[论文](https://db.in.tum.de/~leis/papers/ART.pdf)中的数据结构——ART 树，它是针对 Trie 树的优化。在具体实现上，索引树叶子节点存储了 key 对应的 item，item 中声明了键值对所在的 Datafile、偏移量、和大小。

```go
type Item struct {
	FileID int   `json:"fileid"`
	Offset int64 `json:"offset"`
	Size   int64 `json:"size"`
}
```

这样在查找时，就可以通过文件 id 定位，进而通过偏移量找到在该 Datafile 中的位置，最后通过Size 读取键值对，转换为 Entry。

整个索引树结构也会被存入磁盘，方便从磁盘中恢复读取，而写入磁盘中的结构类似这样：

```
keySize:key:FileID:Offset:Size
```

### 数据文件

数据文件代表存入磁盘的基本结构，一个数据库文件夹下面会有多个 Datafile 文件，这类似 SSTable。

```go
type datafile struct {
	sync.RWMutex // 读写锁

	id           int            // 标识
	r            *os.File       // read文件描述符
	ra           *mmap.ReaderAt // 读取内存映射文件
	w            *os.File       // write文件描述符
	offset       int64          // 偏移量
	dec          *codec.Decoder // decode
	enc          *codec.Encoder // encode
	maxKeySize   uint32         // 最大 key 大小
	maxValueSize uint64         // 最大 value 大小
}
```

`datafile`中包含了文件 id、读写文件描述符、偏移量、编码解码等。在磁盘中的存储方式如下：

```bash
keySize:key:valueSize:value
```

为啥要存一段 key 和 value 的大小？因为每个键值对的大小是不一样，这样从磁盘读取字节段的时候，方便定位 key 和 value。

### 存取数据

一个基本的使用数据库的例子如下：

```go
func main() {
    db, _ := bitcask.Open("/tmp/db")
    defer db.Close()
    db.Put([]byte("Hello"), []byte("World"))
    val, _ := db.Get([]byte("Hello"))
    log.Printf(string(val))
}
```

其中 `Open()` 过程会初始化配置、加载源数据信息、新建索引、新建数据文件等，不详细赘述。来具体看下，存数据和读数据是如何实现的。

`Get()` 和 `Put()` 函数都有一个对应的内部实现，在 `Get`过程会先加读锁，然后调用内部 `get()`函数。

读取数据的过程分为如下几个步骤：

1. 首先从索引树中查找 对应 key 的 item 。
2. 通过 fileID 判断，如果是当前 Datafile，根据 item 中的 offset 和 size，从磁盘读取。
3. 如果不是当前 Datafile，从其他 Datafile 找，读取。
4. 读取到 Entry 之后，校验数据、判断是否过期等，返回。

```go
func (b *Bitcask) get(key []byte) (internal.Entry, error) {
	var df data.Datafile
	// 先从 trie 中查找 key 对应 entry 所在的文件位置
	value, found := b.trie.Search(key)
	if !found {
		return internal.Entry{}, ErrKeyNotFound
	}
	// item，item 是要查找键值对在磁盘中文件位置，偏移量和大小
	item := value.(internal.Item)
	// 如果是当前 datafile
	if item.FileID == b.curr.FileID() {
		df = b.curr
	} else {
		// 不是的话，通过 datafile 数组定位
		df = b.datafiles[item.FileID]
	}
	// 根据 offset 和 entry 的大小，从磁盘中读取 entry，并解码
	e, err := df.ReadAt(item.Offset, item.Size)
	if err != nil {
		return internal.Entry{}, err
	}
	// 如果数据已经过期了，删除该键值对
	if e.Expiry != nil && e.Expiry.Before(time.Now().UTC()) {
		_ = b.delete(key) // we don't care if it doesnt succeed
		return internal.Entry{}, ErrKeyExpired
	}
	// 校验和
	checksum := crc32.ChecksumIEEE(e.Value)
	if checksum != e.Checksum {
		return internal.Entry{}, ErrChecksumFailed
	}
	// 返回数据
	return e, nil
}
```

存数据的过程有如下几个步骤：

1. 首先判断当前 Datafile 是否写满了（超出固定大小限制）。
2. 如果大小超出，先将当前 Datafile 归档（设为只读），然后新建一个 Datafile。
3. 同时保存当前的索引结构，存入磁盘。
4. 如果当前 datafile 够用，构建 Entry， 写入磁盘。

```go
func (b *Bitcask) put(key, value []byte, feature Feature) (int64, int64, error) {
	size := b.curr.Size()
	// 如果当前 datafile 超出
	if size >= int64(b.config.MaxDatafileSize) {
		// 关闭当前 datafile
		err := b.curr.Close()
		if err != nil {
			return -1, 0, err
		}

		id := b.curr.FileID()
		// 将当前 datafile 归档（只读）
		df, err := data.NewDatafile(b.path, id, true, b.config.MaxKeySize, b.config.MaxValueSize, b.config.FileFileModeBeforeUmask)
		if err != nil {
			return -1, 0, err
		}
		// 归入 datafiles 集合
		b.datafiles[id] = df

		// id+1 后新建一个 datafile
		id = b.curr.FileID() + 1
		curr, err := data.NewDatafile(b.path, id, false, b.config.MaxKeySize, b.config.MaxValueSize, b.config.FileFileModeBeforeUmask)
		if err != nil {
			return -1, 0, err
		}
		// 将新建的 datafile 设为 curr
		b.curr = curr
		// 保存 index（当前 datafile 满了后，保存 index）
		err = b.saveIndex()
		if err != nil {
			return -1, 0, err
		}
	}
	// 当前 datafile 够用
	// 构建 entry，写入
	e := internal.NewEntry(key, value, feature.Expiry)
	return b.curr.Write(e)
}
```

在`put()`和`get（）`背后，都使用了 datafile 中的读写文件描述符从磁盘读取和写入，而读写的过程涉及到编码和解码。

以写入为例，`get()` 函数最后的 `b.curr.Wtrie()`，实际执行了一个编码的过程，将对象转为字节序列，最后刷入磁盘中：

```go
func (e *Encoder) Encode(msg internal.Entry) (int64, error) {
	// key 和 value 的大小
	var bufKeyValue = make([]byte, keySize+valueSize)
	// 在 buf 中存入 key 大小字节
	binary.BigEndian.PutUint32(bufKeyValue[:keySize], uint32(len(msg.Key)))
	// 在 buf 存入 value 大小字节
	binary.BigEndian.PutUint64(bufKeyValue[keySize:keySize+valueSize], uint64(len(msg.Value)))
	// 写入 key 和 value 大小
	if _, err := e.w.Write(bufKeyValue); err != nil {
		return 0, errors.Wrap(err, "failed writing key & value length prefix")
	}
	// 写入 key
	if _, err := e.w.Write(msg.Key); err != nil {
		return 0, errors.Wrap(err, "failed writing key data")
	}
	// 写入 value
	if _, err := e.w.Write(msg.Value); err != nil {
		return 0, errors.Wrap(err, "failed writing value data")
	}

	bufChecksumSize := bufKeyValue[:checksumSize]
	binary.BigEndian.PutUint32(bufChecksumSize, msg.Checksum)
	// 写入校验和
	if _, err := e.w.Write(bufChecksumSize); err != nil {
		return 0, errors.Wrap(err, "failed writing checksum data")
	}

	bufTTL := bufKeyValue[:ttlSize]
	if msg.Expiry == nil {
		binary.BigEndian.PutUint64(bufTTL, uint64(0))
	} else {
		binary.BigEndian.PutUint64(bufTTL, uint64(msg.Expiry.Unix()))
	}
	if _, err := e.w.Write(bufTTL); err != nil {
		return 0, errors.Wrap(err, "failed writing ttl data")
	}
	// 上面到 Write 都是写入缓存
	// Flush 同步到磁盘
	if err := e.w.Flush(); err != nil {
		return 0, errors.Wrap(err, "failed flushing data")
	}

	return int64(keySize + valueSize + len(msg.Key) + len(msg.Value) + checksumSize + ttlSize), nil
}
```

同理，解码的过程从磁盘读取字节序列然后转换为 Entry 结构。

另外，在 datafile 归档之后，有一个保存索引的过程，该步骤将当前索引树的结构循环每一个叶子节点，利用编码的方式，写入key 和 item。读取同理。

那利用索引查找 key 对应的 item 的过程？

首先在`Open（）`中，会从磁盘文件加载存储的 index 文件，在加载的过程中，对于从磁盘读取到的索引结构，如果是最新的`indexUpToDate=1`，直接返回。如果索引不是最新的，从 datafile 文件号最大（latest）的开始读取索引（循环读取 kv 并插入 index 树）， 最后从之前所有的 datafile 文件读取。

```go
// 加载index
func loadIndex(path string, indexer index.Indexer, maxKeySize uint32, datafiles map[int]data.Datafile, lastID int, indexUpToDate bool) (art.Tree, error) {
	// 从该路径加载
	t, found, err := indexer.Load(filepath.Join(path, "index"), maxKeySize)
	if err != nil {
		return nil, err
	}
	// 如果加载到了索引，同时索引是最新的，返回
	if found && indexUpToDate {
		return t, nil
	}
	// 如果索引不是最新的，从最新的 datafile 读入索引
	if found {
		if err := loadIndexFromDatafile(t, datafiles[lastID]); err != nil {
			return nil, err
		}
		return t, nil
	}
	// 从之前所有的 datafile 读取索引
	sortedDatafiles := getSortedDatafiles(datafiles)
	for _, df := range sortedDatafiles {
		if err := loadIndexFromDatafile(t, df); err != nil {
			return nil, err
		}
	}
	return t, nil
}
```

### 合并压缩

LSM类型的数据库（比如 LevelDB）一大特点就是数据文件会在后台定期的执行合并压缩操作，以节省空间。该过程会删除掉过期的数据，压缩相同的键值对以保留最新的数据，同时会更新索引结构。

合并压缩的过程会新建一个数据库目录，依次循环对每个 datafile 中的数据进行清理，然后删除老的数据库文件，最后将临时数据库文件重新命名：

```go
func (b *Bitcask) Merge() error {
    ......
    ......
    // 关闭当前 datafile，并设置为只读
	err := b.closeCurrentFile()
    
    // 新打开一个可写 datafile
	err = b.openNewWritableFile()
    
    // 创建一个临时合并文件夹
	temp, err := ioutil.TempDir(b.path, "merge")
    
    // 创建一个临时数据库
	mdb, err := Open(temp, withConfig(b.config))
    
    // 整理key-value 
    err = b.Fold(func(key []byte) error {
		item, _ := b.trie.Search(key)
		// if key was updated after start of merge operation, nothing to do
		if item.(internal.Item).FileID > filesToMerge[len(filesToMerge)-1] {
			return nil
		}
		e, err := b.get(key)
		if err != nil {
			return err
		}
		// prepare entry options
		var opts []PutOptions
		if e.Expiry != nil {
			opts = append(opts, WithExpiry(*(e.Expiry)))
		}

		if err := mdb.Put(key, e.Value, opts...); err != nil {
			return err
		}

		return nil
	})
    
    // 移除旧数据文件
    ......
    err = os.RemoveAll(path.Join(b.path, file.Name()))
    ......
    // 重命名新文件
    for _, file := range files {
		err := os.Rename(
			path.Join([]string{mdb.path, file.Name()}...),
			path.Join([]string{b.path, file.Name()}...),
		)
        ......
}
```



### 总结

待更新。。。

### 参考文献

