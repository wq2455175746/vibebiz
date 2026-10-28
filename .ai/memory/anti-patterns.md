# Anti-Patterns

## AP-001: 前端直连 sandbox
**错误**：`packages/web` 直接请求 `http://localhost:8000/internal/...`
**正确**：前端只调用 backend `/api/v1/*`
**原因**：破坏单向依赖，暴露内部 API，绕过网关校验与反代

## AP-002: 网关解析 Agent 消息
**错误**：Go handler 判断 `payload.type === "tool_use"` 或重组消息
**正确**：按字节 / 行透传 SSE，只处理 HTTP 状态
**原因**：一旦解析，Agent SDK 每次升级都会迫使网关跟版

## AP-003: 用字符串前缀判断路径安全
**错误**：`if !strings.HasPrefix(path, root)` 或 `path.startswith(root)`
**正确**：`Path.resolve()` 后 `is_relative_to(project_dir)`
**原因**：`../`、符号链接、大小写/分隔符差异都能绕过字符串前缀

## AP-004: 前端持有权威代码副本
**错误**：把项目文件长期存进 Pinia 并以其为准
**正确**：打开时从 sandbox 读取，保存带 `baseVersion`，冲突时重新加载
**原因**：真相源分裂，AI 改动与用户改动会互相覆盖

## AP-005: SSE 整段缓冲
**错误**：Go 先 `io.ReadAll` 再一次性写响应；nginx 默认 proxy_buffering
**正确**：逐行读取 + `Flush()`；反代关闭 buffering / `FlushInterval=-1`
**原因**：聊天流失去实时性，表现为一次性输出

## AP-006: 首个 `[DONE]` 就关闭业务流
**错误**：把 `[DONE]` 当成 HTTP 连接终止信号并清理所有状态
**正确**：`[DONE]` 表示本轮结束；连接由读取结束或 context 取消关闭
**原因**：多轮 Agent 场景会提前中断后续轮次

## AP-007: 生产环境预览与主站同 origin
**错误**：iframe 与主站同域同 origin，并给 `allow-same-origin`
**正确**：独立预览子域 + sandbox 白名单 + `referrerpolicy=no-referrer`
**原因**：用户生成代码可读取主站 cookie / storage

## AP-008: 把 LLM key 放进前端或用户工程
**错误**：`VITE_LLM_API_KEY`、写入 `.env` 到工作区
**正确**：key 只在 sandbox 服务端环境变量；前端只与本平台通信
**原因**：浏览器与生成代码均可读，导致密钥泄漏

## AP-009: 用 `model.setValue()` 应用 AI 增量
**错误**：Monaco 每次 AI 改动直接 `setValue` 覆盖全文
**正确**：`editor.executeEdits()` 走 undo 栈，保留用户 Ctrl+Z
**原因**：`setValue` 清空撤销历史，用户失去回退能力

## AP-010: 在网关做压缩 / 大文件搬运
**错误**：网关 gzip、转发 50MB 文件正文
**正确**：压缩下沉到数据面；大文件走 Range / 元数据闸门
**原因**：网关是单一收敛点，字节搬运会拖慢同机所有用户

## AP-011: 把 M0 stub 当成真实生成能力
**错误**：README / UI 宣称已能生成应用，实际只是确定性改写
**正确**：明确标注 `StubAgent`，M1 才接入真实 LLM
**原因**：误导验收，掩盖未完成项

## AP-012: 引入设计文档以外的数据库 / 中间件
**错误**：harness 阶段上 MySQL、Redis、消息队列
**正确**：M0 内存 store + 磁盘工作区；M3 按设计文档引入持久化
**原因**：48 小时目标下增加部署与迁移成本，偏离主流程验证

## AP-013: 页面使用黑白灰蓝以外的颜色
**错误**：模板卡片、状态标签使用绿 / 紫 / 橙等
**正确**：仅黑 / 白 / 灰 / 蓝；确需新增先向产品确认
**原因**：产品明确的视觉约束，参考 Atoms 主流程也需要统一

## AP-014: 未验证就宣称 Docker 可用
**错误**：没有 docker CLI 却报告「镜像构建通过」
**正确**：交付 Dockerfile 并标注「未在本机构建验证」
**原因**：验证必须基于实际命令输出，不能凭代码推断
