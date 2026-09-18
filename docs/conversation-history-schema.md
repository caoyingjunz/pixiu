# 对话历史表结构

沿用现有 `conversations` 和 `messages`，不重复创建聊天表。
`messages` 已保存 `conversation_id`、`input_text`、`output_text`、成功状态和 token 用量；
当前每条记录是一轮问答，不改为按 role 拆分，避免影响已有聊天写入链路。

本次新增：

| 表 | 字段 | 含义 |
| --- | --- | --- |
| conversations | user_id，可空 BIGINT | 会话所属用户 |
| conversations | last_message_at，可空 DATETIME | 最近消息时间 |
| users | last_conversation_id，可空 BIGINT | 上次选中的会话 |

新增索引 `conversations(user_id, last_message_at)` 和 `users(last_conversation_id)`。
主键、创建时间、更新时间继续使用现有 `id`、`gmt_create`、`gmt_modified`。

## 迁移

User、Conversation 和 Message 已注册到 GORM AutoMigrate。
启用 `default.auto_migrate` 后，服务启动会补齐字段及索引；本次代码修改不直接操作运行中的数据库。
新增字段均允许 NULL，因此旧用户和旧会话无需伪造归属，也不会破坏现有写入。
历史会话的 user_id 只有在可靠确认归属后才能回填；不得将无归属记录当成公共历史展示。
沿用项目现有标量 ID 关联方式，本次不增加数据库外键或级联删除。

## 接口与页面行为

- GET `/pixiu/assistant/conversations/current`：返回本人上次选择的会话；无选择或已失效返回 null。
- PUT `/pixiu/assistant/conversations/current`，请求体 `{"conversation_id": 123}`：验证归属后保存选择。传 0 表示新建空白对话。
- GET `/pixiu/assistant/conversations`：分页返回本人会话，按最近消息时间及 ID 倒序排列。
- 原有会话详情、删除、续聊和消息接口均限制为当前用户的会话。
- 创建会话和保存用户指针在同一事务内完成；删除会话时清空指针。旧消息保留，但不再通过用户消息接口返回。

集群智能助手浮窗打开时恢复当前会话，并从 history 按原顺序显示问答。
顶部可切换历史会话、加载更多历史或新建空白对话。新建空白选择立即保存，首次发送时、调用工具前创建会话。
生成中禁用切换和新建，历史加载失败时禁止发送并提供重试入口，避免误建会话。
浮窗恢复问答文本；“智能助手 / 对话记录”的详情提供执行记录标签，分页展示已保存的工具调用。
GET `/pixiu/assistant/conversations/:conversationId/executions` 校验会话所有权后返回执行记录。
新会话在工具调用前分配 ID，失败回复前已完成的操作也可查询。历史上未关联会话 ID 的执行记录无法可靠自动回填；较长工具输出沿用现有截断策略，参数完整保存。

未成功完成的生成不加入会话 history；失败请求继续沿用 messages 审计记录。
已有无 user_id 的历史会话不会自动归属给任何用户。
