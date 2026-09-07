-- jfs-router 的默认 MySQL 元数据表结构。
-- 表名/列名都可以通过 config.json 里 metastore.mysql.{table,segment_column,
-- backend_column} 自定义，不强制要求叫这个名字——如果你们已经有类似的
-- 表，直接在配置里指过去即可，不需要迁移数据。

CREATE TABLE IF NOT EXISTS mount_routes (
    segment    VARCHAR(255)  NOT NULL PRIMARY KEY,
    backend    VARCHAR(1024) NOT NULL,
    updated_at DATETIME      DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 示例数据
INSERT INTO mount_routes (segment, backend) VALUES
    ('tenantA', '/mnt/.jfs-tenantA'),
    ('tenantB', '/mnt/.jfs-tenantB');
