-- 1. 资源池主表
CREATE TABLE resource_pools (
    pool_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    name VARCHAR(64) NOT NULL UNIQUE,

    description TEXT NOT NULL DEFAULT '',

    status VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. 资源池与节点关联表
CREATE TABLE pool_nodes (
    -- node_id 作为主键，确保一个节点只能加入一个资源池
    node_id TEXT PRIMARY KEY
        REFERENCES nodes(node_id)
        ON DELETE CASCADE,

    pool_id BIGINT NOT NULL
        REFERENCES resource_pools(pool_id)
        ON DELETE CASCADE,

    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 按资源池查询节点
CREATE INDEX idx_pool_nodes_pool_id
    ON pool_nodes(pool_id);

-- 按创建时间查询资源池
CREATE INDEX idx_resource_pools_created_at
    ON resource_pools(created_at DESC);