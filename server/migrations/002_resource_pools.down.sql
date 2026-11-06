-- 先删除节点关联表，再删除资源池主表
DROP TABLE IF EXISTS pool_nodes;
DROP TABLE IF EXISTS resource_pools;