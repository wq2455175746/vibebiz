# Ops 模型录入设计

**日期：** 2026-10-29  
**状态：** 待实现  
**范围：** ops-web / CP / MySQL `llm_model`  
**不包含：** 编辑、删除、启停、模型连通性测试、sandbox 模型切换

## 1. 背景与目标

ops-web 目前没有模型管理入口，`llm_model` 表只有 DDL 和 DAO model，没有 admin 接口和录入界面。本期提供最小可用的平台模型录入能力：

- 运营可在 ops-web 查看平台模型列表。
- 可新增 LLM、EMBEDDING、RERANK、OCR、ASR、TTS、OTHER 类型模型。
- 录入的 API Key 加密后写入 `llm_model.api_key_cipher`。
- 默认模型按 `model_type` 唯一。

## 2. 非目标

- 不做编辑、删除、启用/禁用。
- 不做模型连通性测试或自动发现模型列表。
- 不支持用户级 BYOK（`scope=user`）。
- 不把模型接入 sandbox 或聊天链路。
- 不修改 `llm_model` 表结构；`model_type` 为 `varchar(32)`，直接支持新类型。

## 3. 职责边界

| 层 | 职责 |
|---|---|
| ops-web | 模型列表、新增表单、客户端校验、错误展示 |
| CP | admin 鉴权、字段校验、API Key 加密、写入 `llm_model` |
| MySQL | 平台模型持久化、唯一约束、默认模型互斥 |
| sandbox | 不参与 |

## 4. 数据模型

写入 `llm_model` 的固定字段：

| 字段 | 值 |
|---|---|
| `scope` | `platform` |
| `user_id` | `0` |
| `model_name` | 用户输入 |
| `model_code` | 用户输入，平台内唯一 |
| `api_url` | 用户输入 |
| `api_key_cipher` | AES-GCM 密文 |
| `provider_code` | `OpenAI` / `DeepSeek` / `Anthropic` / `Qwen` / `Other` |
| `model_type` | `LLM` / `EMBEDDING` / `RERANK` / `OCR` / `ASR` / `TTS` / `OTHER` |
| `max_tokens` | LLM 使用表单值，默认 4096；其他类型写 0 |
| `is_default` | 0/1；同一 `model_type` 只允许一个 1 |
| `status` | 1 启用 / 0 禁用，默认 1 |
| `extra_params` | 可选 JSON 对象；为空写 NULL |
| `created_by` | 当前 ops 用户名 |

`is_small_model`、`established`、`context_window` 本期不录入，使用表默认值。

## 5. API 契约

### 5.1 列表

```text
GET /cp/api/v1/admin/models
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "models": [
      {
        "id": 1,
        "modelName": "DeepSeek Chat",
        "modelCode": "deepseek-chat",
        "apiUrl": "https://api.deepseek.com/anthropic",
        "providerCode": "DeepSeek",
        "modelType": "LLM",
        "maxTokens": 4096,
        "isDefault": true,
        "status": 1,
        "hasApiKey": true,
        "extraParams": null,
        "createdAt": "2026-10-29T12:00:00Z",
        "updatedAt": "2026-10-29T12:00:00Z"
      }
    ]
  }
}
```

- 只返回 `scope=platform`、`user_id=0` 且未软删除的记录。
- 永不返回 `api_key_cipher`，只返回 `hasApiKey`。
- 按 `created_at DESC` 排序。

### 5.2 新增

```text
POST /cp/api/v1/admin/models
```

请求：

```json
{
  "modelName": "DeepSeek Chat",
  "modelCode": "deepseek-chat",
  "apiUrl": "https://api.deepseek.com/anthropic",
  "apiKey": "sk-...",
  "providerCode": "DeepSeek",
  "modelType": "LLM",
  "maxTokens": 4096,
  "isDefault": true,
  "status": 1,
  "extraParams": { "voice": "alloy" }
}
```

成功返回创建后的模型对象（同列表项）。

错误：

| 场景 | HTTP | code |
|---|---|---|
| 必填缺失、编码格式错误、类型/供应商非法、JSON 非法 | 400 | 40001 |
| `model_code` 已存在 | 409 | 40901 |
| `MODEL_SECRET_KEY` 未配置或非法 | 503 | 50301 |
| 其他数据库错误 | 500 | 50001 |

## 6. 加密

- 新增 `packages/backend/internal/secret`。
- `NewCipher(encoded string)` 接收 base64 编码的 32 字节密钥。
- `Encrypt(plaintext)` 使用 AES-GCM，随机 nonce，输出 `base64(nonce+ciphertext)`。
- `Decrypt` 同步实现，便于测试和后续使用。
- 密钥来源：CP 环境变量 `MODEL_SECRET_KEY`。
- 未配置时 CP 仍可启动；新增模型返回 503。

## 7. 后端结构

### 7.1 `internal/llmmodel`

- `Model`：对外模型视图，不含密钥。
- `CreateInput`：包含明文 `APIKey`、`ExtraParams json.RawMessage`。
- `Repository`：
  - `List(ctx) ([]Model, error)`
  - `Create(ctx, input, apiKeyCipher, createdBy) (Model, error)`
- `Service`：
  - 校验必填、编码格式、类型、供应商、状态、`extra_params`。
  - 加密 API Key。
  - 未配置 cipher 时返回 `ErrKeyMissing`。
- `MySQLRepository`：
  - 事务内先清除同 `model_type` 的旧默认，再插入。
  - MySQL 1062 映射为 `ErrConflict`。
- `MemoryRepository`：测试和内存模式使用。

### 7.2 Handler 与路由

- `handler.ModelHandler` 使用 `ModelService` 接口。
- 新增 `GET /cp/api/v1/admin/models` 与 `POST /cp/api/v1/admin/models`。
- 注册在 `adminProtected` 下，沿用 ops 鉴权和审计中间件。
- `RegisterControl` 增加 `models *handler.ModelHandler` 参数；为 nil 时不注册。

### 7.3 配置

- `config.Config` 增加 `ModelSecretKey string`，读取 `MODEL_SECRET_KEY`。
- `packages/backend/.env.example` 增加：

```text
# base64 编码的 32 字节密钥，用于加密 llm_model.api_key_cipher
MODEL_SECRET_KEY=
```

## 8. ops-web

### 8.1 路由与导航

- 新增 `ModelsView.vue`。
- 路由 `/models`，名称 `models`，标题“模型管理”。
- 导航项插入“沙箱节点”之后、“配额”之前，图标 `pi pi-box`。

### 8.2 列表

列：模型名称、编码、类型、供应商、API URL、最大 tokens、默认、状态、创建时间。

- 默认列显示“是 / 否”。
- 状态列显示“启用 / 禁用”。
- `maxTokens=0` 显示“—”。

### 8.3 新增表单

Dialog 字段：

- 模型名称：必填。
- 模型编码：必填，`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`。
- 模型类型：下拉，7 类，默认 LLM。
- 供应商：下拉，默认 Other。
- API URL：必填，`http://` 或 `https://` 开头。
- API Key：必填，密码输入。
- 最大 tokens：仅 LLM 显示，默认 4096，必须大于 0。
- 是否默认：开关/复选框，默认关闭。
- 状态：启用/禁用，默认启用。
- extra_params：可选 JSON 文本，必须为对象。

客户端校验通过后调用 `POST /admin/models`；成功后关闭弹窗、刷新列表、显示“模型已创建”。

## 9. 测试

### Go

- `secret`：加解密往返、随机 nonce、非法密钥。
- `llmmodel.Service`：必填校验、编码格式、类型/供应商、extra_params、加密调用、未配置 cipher。
- `llmmodel.MemoryRepository`：同类型默认互斥、重复编码冲突。
- `handler.ModelHandler`：列表、新增、400/409/503 映射、响应不含密钥。
- `router`：admin 路由存在，CP 以外不注册。

### ops-web

- `api/admin`：`getModels` / `createModel` 信封解包。
- `utils/modelForm`：必填、编码、URL、JSON、LLM tokens、非 LLM tokens 归零。
- 路由与导航测试更新为 9 个模块。

## 10. 验收

- ops-web 侧栏出现“模型管理”，列表能展示 `llm_model` 平台模型。
- 新增 LLM / EMBEDDING / ASR / TTS 等类型模型成功写入数据库。
- API Key 以密文保存，列表和接口不返回明文。
- 同类型设置默认时旧默认自动取消；不同类型默认互不影响。
- 重复 `model_code` 返回 409。
- `make test`、`make quality`、`make validate` 通过。
