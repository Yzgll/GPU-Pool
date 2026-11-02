
-- 1. 节点主表：保存节点最新状态
CREATE TABLE nodes (
    node_id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'online'
        CHECK (status IN ('online', 'offline')),
    collected_at TIMESTAMPTZ NOT NULL,
    last_heartbeat TIMESTAMPTZ NOT NULL,
    latest_snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 按心跳时间查询节点
CREATE INDEX idx_nodes_last_heartbeat
    ON nodes (last_heartbeat DESC);

-- 按在线状态查询节点
CREATE INDEX idx_nodes_status
    ON nodes (status);


-- 2. GPU 设备表：保存节点当前 GPU 硬件信息
CREATE TABLE gpu_devices (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(node_id)
        ON DELETE CASCADE,
    gpu_uuid TEXT NOT NULL,
    gpu_index INTEGER NOT NULL CHECK (gpu_index >= 0),
    name TEXT NOT NULL,
    memory_total_mb BIGINT NOT NULL DEFAULT 0,
    driver_version TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (node_id, gpu_uuid),
    UNIQUE (node_id, gpu_index)
);

-- 按节点查询 GPU
CREATE INDEX idx_gpu_devices_node_id
    ON gpu_devices (node_id);


-- 3. 资源快照表：保存每次心跳对应的资源历史
CREATE TABLE resource_snapshots (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(node_id)
        ON DELETE CASCADE,
    collected_at TIMESTAMPTZ NOT NULL,
    system_info JSONB NOT NULL,
    gpu_info JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (node_id, collected_at)
);

-- 按节点和采集时间查询历史资源
CREATE INDEX idx_resource_snapshots_node_time
    ON resource_snapshots (node_id, collected_at DESC);

-- 按采集时间查询快照
CREATE INDEX idx_resource_snapshots_collected_at
    ON resource_snapshots (collected_at DESC);
