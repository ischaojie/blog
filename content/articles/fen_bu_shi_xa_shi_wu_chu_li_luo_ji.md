+++
title = "分布式XA事务处理逻辑"
date = 2020-08-21T22:00:04+08:00
draft = false
[taxonomies]
tags = ["分布式", "learn"]
+++


事务在数据库中代表一系列操作要么全部都完成，要么全部都失败，ACID规定了事务操作的原子性、一致性、隔离性和持久性。然而数据库的环境不可能只在单机上，在分布式环境下，一个事务中某个操作可能发往A节点，而另一个操作发往B节点，这就导致无法保证ACID的原则。

实现分布式事务常见的解决办法有以下几种：XA两阶段提交协议、TCC协议和SAGA协议。但是这些解决办法都不可能完全保证事务不出错。分布式系统中有一个CAP定理，说的是在分布式情况下，不可能同时满足一致性、可用性和容错性这三个条件，一般需要满足其中两个条件。

## XA两阶段提交协议

XA协议规定了分布式事务的标准，其中 **AP** 代表应用程序，**TM** 代表事务管理器，负责协调和管理事务，而**RM** 代表着资源管理器。

![image-20200723085002817](C:\Users\admin.MENGFANDE3-PC\AppData\Roaming\Typora\typora-user-images\image-20200723085002817.png)

而事务的具体处理过程就是TM和RM之间的交互，分为两个阶段：

第一阶段：事务管理器要求每个涉及到事务的数据库预提交(precommit)此操作，并反映是否可以提交。

第二阶段：事务管理器要求每个数据库提交数据，或者回滚数据。



以MySQL中的XA处理逻辑为例（MySQL5.7版本实现了对XA协议的支持），来看下这两个阶段的逻辑处理过程。

对于一个事务：

```sql
begin;
insert into student values ('xiaoming', 18);
update test set age = 18 where name = 'xiaoming';
commit;
```

### **第一阶段**

事务管理器会生成一个全局的事务ID，比如使用uuid生成一个唯一的ID，为了方便用 **xid1** 代替。

首先，遇到 **begin**，不处理。

然后是 **insert** 操作，事务管理器根据表中主键的值计算（hash）出应该分布在哪个节点上，比如insert语句被计算出应该发到节点A上，事务管理器就像A节点发送命令开始XA事务，同时将insert语句发送过去。

```sql
xa start 'xid1';  # 开启事务
insert into student values ('xiaoming', 18);
```

接下来 **update** 操作，同样的，事务管理器根据主键计算所属节点，开启XA，发送update语句。

```sql
xa start 'xid1'; 
update test set age = 18 where name = 'xiaoming';
```

**commit** 的时候，事务管理器分别向节点A和B发送一个预提交操作：

```sql
xa end 'xid1';
xa prepare 'xid1';
```

### 第二阶段

如果节点A和B都返回就绪ready，此时进入 **第二阶段**：

事务管理器分别向节点AB发送commit操作：

```sql
xa commit 'xid1';
```

相反的，如果有任何一个节点是unready，事务管理器就会通知A、B节点的操作回滚：

```sql
xa rollback'xid1';
```

有一个问题，如果在进入第二阶段commit的时候，某个数据节点出现故障，会导致节点状态不一致。解决办法是把XA事务处理的过程也存入日志数据，比如MySQL将其写入了binlog，这样在出现问题时还可以恢复。

整个XA的过程：

```sql
# 阶段一
xa start 'xid1';  
insert into test values (1, 1);

xa start 'xid1'; 
update test set b = 1 where a = 10;

xa end 'xid1';
xa prepare 'xid1';

# 阶段二
xa commit 'xid1'; 
# or
xa rollback 'xid1'; # 失败回滚
```

## EverDB分布式事务的支持

### MyCat中的实现

EDB-Grid组件中，借鉴了MyCat（也是一个数据库中间件）的XA处理逻辑，MyCat根据XA协议实现了对分布式事务的支持，具体来说：

通过数据库编程接口（比如JDBC，也就是XA协议中的AP）开启XA事务，然后执行SQL语句，预提交，最后commit。

```java
 // 开始XA事务
 conn.prepareStatement("set xa=on").execute();

// 插入语句
// 分别预提交
conn.prepareStatement(sql1).execute();
conn.prepareStatement(sql2).execute();

// commit
 conn.commit();
```

过程跟MySQL类似，在实现上，利用uuid生成了一个全局的事务ID：

```java
public void setXATXEnabled(boolean xaTXEnabled) {
   if (xaTXEnabled) {
       if (this.xaTXID == null) {
           xaTXID = genXATXID(); // 获得 XA 事务编号
       }
   } else {
       this.xaTXID = null;
   }
}
//......
public static String getUUID() {
   String s = UUID.randomUUID().toString();
   return s.substring(0, 8) + s.substring(9, 13) + s.substring(14, 18) + s.substring(19, 23) + s.substring(24);
}
```

然后在事务管理器向节点分发语句时，会先写入XA START：

```java
if (expectAutocommit == false && xaTxID != null && xaStatus == TxState.TX_INITIALIZE_STATE) { 
       xaCmd = "XA START " + xaTxID + ';';
       this.xaStatus = TxState.TX_STARTED_STATE;
   }

//......

// and our query sql to multi command at last
sb.append(rrn.getStatement() + ";");
// syn and execute others
this.sendQueryCmd(sb.toString());
```

MyCat在执行事务操作是，会同时将其写入日志中，保证可恢复。

```java
if (mysqlCon.getXaStatus() == TxState.TX_STARTED_STATE) { // XA 事务
               //recovery Log
               participantLogEntry[started] = new ParticipantLogEntry(xaTxId, conn.getHost(), 0, conn.getSchema(), ((MySQLConnection) conn).getXaStatus());
               String[] cmds = new String[]{"XA END " + xaTxId, // XA END 命令
                       "XA PREPARE " + xaTxId}; // XA PREPARE 命令
               mysqlCon.execBatchCmd(cmds);
```

同样的，commit时也会同步写入日志。

rollback：

```java
if (needRollback) {
           for (int j = 0; j < coordinatorLogEntry.participants.length; j++) {
               ParticipantLogEntry participantLogEntry = coordinatorLogEntry.participants[j];
               //XA rollback
               String xacmd = "XA ROLLBACK " + coordinatorLogEntry.id + ';';
               OneRawSQLQueryResultHandler resultHandler = new OneRawSQLQueryResultHandler(new String[0], new XARollbackCallback());
               outloop:
               // ...
```

### EverDB中的实现

再来看下EverDB的处理过程：

首先是生成xid，从0开始递增。

```c++
unsigned long XA_manager::generate_xid()
{
  unsigned long ret = 0;
  xid_mutex.acquire();
  try {
    //TODO: find a place to do init_max_xid
    if (!init_xid)
      init_max_xid();
    ret = xid_next++;
    if (!ret) // 0 is kept as the initial value
      ++ret;
      //...
```

开始XA事务：

```c++
void MySQLXA_helper::init_conn_to_start_xa(Session *session,
                                           DataSpace *space,
                                           Connection *conn)
{
  unsigned long xid = session->get_xa_id();

  // clear the pending transaction
  conn->execute_one_modify_sql("COMMIT;");

  // ......
  
    record_xa_redo_log(session, space, sql.c_str());  // log
   
  }
    
  // ...
    
  // start xa transaction
  sql += "XA START '";
  sql += tmp;
  sql += "';";
  conn->execute_one_modify_sql(sql.c_str());
  conn->set_start_xa_conn(true);
}
```



第二阶段：XA COMMIT 或者ROLLBACK：

```c++
void xa_commit_or_rollback_xid(Connection *conn, string xid, int flag)
{
  string sql("");
  if (flag == TC_TRANSACTION_COMMIT)
    sql += "XA COMMIT '"; // xa commit
  else if (flag != TC_TRANSACTION_COMMIT)
    sql += "XA ROLLBACK '"; // xa rollback

  sql += xid.c_str();
  sql += "';";

  check_xa_sql_is_not_running(conn, sql);
  TimeValue timeout = TimeValue(backend_sql_net_timeout);
  //......
  }
}
```

同时事务处理的过程会写入redolog中，比如上面的开始XA事务中 **record_xa_redo_log** 。






