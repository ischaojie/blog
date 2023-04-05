+++
title = "分布式NewSQL数据库简介"
date = 2020-09-28T15:34:42+08:00
draft = false
[taxonomies]
tags = ["分布式", "数据库"]
+++

数据库的发展经历了从传统的关系型数据库、NoSQL（Not Only SQL）数据库到近几年新出现的分布式 NewSQL 数据库，整个趋势由单机逐渐向分布式方向发展。关系型数据库自1970年由 Edgar Codd 提出以来[1]，在相当长的一段时间内，成为市场占有量最大的数据库产品。除此之外，网络型数据库和分层型数据库也在一段时间内短暂出现过。

然而关系型数据库有自身的不足之处，其数据关系模型表达与实际应用层之间存在不连贯性，且无法高效的扩展到多个节点，以及对于大数据量、高吞吐量的写入支持有限等。 NoSQL 的出现旨在解决这些不足，NoSQL 被解释为“不仅仅是 SQL ”，以 Google 的BigTable[2] 和 Amazon 的 DynamoDB[3] 为代表，其在模型上更加灵活，包括文档、键值、列族、图等多种数据模型。以开源 NoSQL 数据库 MongoDB 为例，其数据模型以文档作为基本结构，文档中可任意存放键值对，数据模型由存入数据的结构决定。

在数据关系的表达上，NoSQL 对于一对多关系有更强的灵活性，对查询更友好，不需要跨表连接，但是对于多对多关系，两种数据库并没有多大不同。除此之外，NoSQL 数据库往往针对可扩展性、高可用性等专门设计，这使得其支持更复杂的多数据中心架构，性能也更强。然而传统的关系型数据库近些年在其数据模型和高可用性等方面也添加了相应支持，比如开源数据库 PostgreSQL 在其 9.3 版本之后，添加了对文档模型的支持。在高可用上，MySQL 也支持主从复制以及自 5.7 版本之后出现的 MySQL Group Replication 技术，进一步增强了在扩展性和高可用性上的支持。可以说，关系型数据库和 NoSQL 数据库相互借鉴各自的优点，协同发展，呈现出混合持久化的状态。

分布式NewSQL数据库的出现基于可扩展性、高可用性、数据一致性等方面的考虑。其中可扩展性是指水平方向（垂直扩展指的是扩展单机，以共享内存或者共享磁盘的方式存在）的扩展，将单台机器的负载分散到多台机器上，提供更强的处理能力。高可用保障了在单台机器出现故障的情况下，系统仍能继续提供服务。数据一致性作为分布式事务的必要条件，保证了所有节点对某个事件达成一致。NewSQL 概念的产生来源于 Google 于 2012 年发表的Spanner[4] 数据库，该论文将传统关系模型以及 NoSQL 数据库的扩展性相结合，使得数据库同时支持分布式又具有传统 SQL 的能力。除了 Spanner 数据库，国外的CockroachDB 以及国内的 TDSQL、MyCAT、TiDB[5]、OceanBase、SequoiaDB 等都是新兴的分布式数据库产品。

分布式数据库从实现上可以分为三类：

- 一类是以传统数据库组成集群，利用主从复制等实现分布式，比如 MySQL 集群方案
- 一类是在现有数据库之上以中间件代理的形式，提供自动分库分表、故障切换、分布式事务等支持，以 MyCAT、TDSQL 等为代表。
- 一类是原生的分布式架构，通过共识算法实现高可用性、扩展性、数据一致性等支持，以 TiDB、OceanBase 等为代表。

分布式 CAP 理论指出[6]，一个分布式系统不可能同时满足一致性、可用性和分区容错性这三个特点，其中分区容错性无法避免，势必要在一致性和可用性中做出权衡。

可用性的保证可以通过**复制**技术实现，通过在多台机器上保存数据副本，提高系统可用性和读取吞吐量。MyCAT 以及 TDSQL 均支持主从复制的方式。传统主从复制方式的问题在于无法保证数据的强一致性，如果主库故障，可能会出现多个节点成为主库（脑裂问题），导致数据丢失或损坏。MySQL 在 5.7 版本推出了 MySQL Group Replication 功能，实现了基于 Paxos 共识算法的高可用性和数据强一致性保证，TiDB 基于 Raft 共识算法[7]保证了数据的强一致。

分布式数据库的另一个特点是对**事务**的支持，分布式场景下保障事务的 ACID 原则常见的办法有 2PC 协议、TCC 协议以及 SAGA 协议，TiDB、OceanBase 等均使用两阶段提交协议（2PC）来实现跨多个节点的事务提交。

动态扩展的实现可使用分区的方式，将原有单个节点的压力分散到多个节点，提升系统性能。分区面临的问题是如何将数据和查询负载均匀分布在各个节点，常见的解决办法有基于 Hash 的分区和基于 Range 的分区，TiDB 使用 Range 的方式分区，而 OceanBase 两种都支持。除了对以上特点的支持，分布式数据库还具有 HTAP、SQL 引擎、兼容性等特点。

参考文献：

[1] Edgar F. Codd: “A Relational Model of Data for Large Shared Data Banks,” Communications of the ACM, volume 13, number 6, pages 377–387, June 1970. 

[2] CHANG, Fay, DEAN, et al. Bigtable : A Distributed Storage System for Structured Data[J]. Acm Transactions on Computer Systems, 2008, 26(2):1-26.

[3] Decandia G, Hastorun D, Jampani M, et al. Dynamo: Amazon's Highly Available Key-value Store[J]. Acm Sigops Operating Systems Review, 2007, 41(6):205-220.

[4] J. C. Corbett, J. Dean, M. Epstein, A. Fikes, et al. Spanner: Google’s Globally Distributed Database. ACMTrans. Comput. Syst., 31(3):8:1–8:22, 2013.

[5] Dongxu Huang, Qi Liu, Qiu Cui, Zhuhe Fang, Xiaoyu Ma, Fei Xu, Li Shen, Liu Tang, Yuxing Zhou, Menglong Huang, Wan Wei, Cong Liu, Jian Zhang, Jianjun Li, Xuelian Wu, Lingyu Song, Ruoxi Sun, Shuaipeng Yu, Lei Zhao, Nicholas Cameron, Liquan Pei, Xin Tang. TiDB: A Raft-based HTAP Database. PVLDB, 13(12): 3072-3084, 2020.

[6] Seth Gilbert and Nancy Lynch: “Perspectives on the CAP Theorem,” IEEE Computer Magazine, volume 45, number 2, pages 30–36, February 2012.

[7] Heidi Howard, Malte Schwarzkopf, Anil Madhavapeddy, and Jon Crowcroft: “Raft Refloated: Do We Have Consensus?,” ACM SIGOPS Operating Systems Review, volume 49, number 1, pages 12–21, January 2015. doi:10.1145/2723872.2723876



