# 供号机卡密入库 API

## 1. 用途

供号机可以通过本接口把卡密批量写入站点库存。每个供号机来源（source）在后台固定绑定一个商品 SKU：

```text
一个 API Key  →  一个当前绑定的 SKU
```

调用方不需要、也不能在请求体中传入商品 ID 或 SKU ID。服务端根据 API Key 对应的 source 自动决定入库目标，并沿用现有卡密库存和自动发货流程。

接口地址：

```text
POST /api/v1/card-supply/secrets
```

请求体最大 2 MiB。source 的单批最大数量由后台配置，默认 500，允许范围为 1–5000。

## 2. 后台创建凭证

管理员进入后台的 **商品管理 → 供号机 API**：

1. 创建一个 source，填写名称；
2. 选择商品和 SKU；
3. 按需填写批量上限和 IP 白名单；
4. 保存后记录页面一次性显示的 `API Key` 和 `API Secret`；
5. 将这两个值安全配置到供号机中。

注意：

- `API Secret` 只在创建或重置时返回一次，列表和详情接口不会再次返回明文；
- API Secret 不要提交到代码仓库、日志或工单；
- 后台重置 API Secret 后，旧 Secret 立即失效，供号机必须同步更新；
- 禁用 source 后，该 source 的请求会被拒绝；
- 删除 source 是软删除，原 API Key 不再可用。

## 3. 请求头

每次请求必须带以下请求头：

| Header | 说明 |
| --- | --- |
| `Dujiao-Next-Api-Key` | 后台生成的 API Key |
| `Dujiao-Next-Timestamp` | Unix 时间戳，单位为秒 |
| `Dujiao-Next-Signature` | HMAC-SHA256 签名，小写十六进制字符串 |
| `Content-Type` | `application/json` |

服务器接受的时间戳与服务器当前时间最大偏差为 **60 秒**。供号机应使用 NTP 或其他方式保持系统时间准确。

## 4. 签名算法

签名使用 API Secret 作为 HMAC-SHA256 密钥。签名原文为：

```text
{HTTP_METHOD}\n{REQUEST_PATH}\n{TIMESTAMP}\n{MD5_OF_RAW_BODY}
```

其中：

- `HTTP_METHOD`：大写 HTTP 方法，例如 `POST`；
- `REQUEST_PATH`：只使用路径，不包含域名、协议和 query string，本接口固定为 `/api/v1/card-supply/secrets`；
- `TIMESTAMP`：请求头中的 Unix 秒级时间戳原值；
- `MD5_OF_RAW_BODY`：实际发送的原始 HTTP body 的 MD5 小写十六进制值；
- `\n`：一个 LF 换行符（字节 `0x0a`）。

伪代码：

```text
body_md5 = md5(raw_body).hex()
sign_string = METHOD + "\n" + PATH + "\n" + str(timestamp) + "\n" + body_md5
signature = hex_lower(hmac_sha256(api_secret, sign_string))
```

**签名计算完成后，发送的 body 必须与计算签名时的原始字节完全一致。** 不要先对 JSON 签名、再由 HTTP 客户端重新格式化 JSON；空格、换行、字段顺序和 Unicode 编码变化都会导致签名不同。

Go 示例：

```go
package main

import (
    "fmt"
    "time"

    "github.com/dujiao-next/internal/upstream"
)

func main() {
    timestamp := time.Now().Unix()
    body := []byte(`{"request_id":"machine-01-20260927-000001","secrets":["账号001:密码001"],"note":"daily import"}`)

    signature := upstream.Sign(
        "YOUR_API_SECRET",
        "POST",
        "/api/v1/card-supply/secrets",
        timestamp,
        body,
    )
    fmt.Println(timestamp, signature)
}
```

如果供号机不是 Go 实现，只要严格按上面的原文格式计算即可。

## 5. 请求体

```json
{
  "request_id": "machine-01-20260927-000001",
  "secrets": [
    "账号001:密码001",
    "账号002:密码002"
  ],
  "note": "可选备注"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `request_id` | string | 是 | 供号机侧唯一请求 ID，最多 128 个字符；建议使用机器编号、日期和递增序号组合 |
| `secrets` | string[] | 是 | 卡密数组，至少一项；每个数组元素是一条完整卡密文本 |
| `note` | string | 否 | 批次备注 |

卡密约束：

- 卡密是**不透明的完整文本**，服务端不会拆分账号和密码；
- `账号:密码` 中的冒号会原样保留；`|`、空格及其他字符也会原样保留；
- `secrets` 数组中的一项对应一条卡密，不要把多条卡密拼成一个元素；
- 单个卡密不能包含 `\\r` 或 `\\n`，否则请求会被拒绝；
- 外层首尾空白会被去除，实际库存内容以去除首尾空白后的文本保存；
- 同一请求内重复的卡密只保留第一条。

## 6. curl 请求示例

下面示例假设 `BODY` 文件保存的就是实际发送的 JSON 原始字节：

```bash
API_KEY='your-api-key'
API_SECRET='your-api-secret'
TIMESTAMP="$(date +%s)"
PATH='/api/v1/card-supply/secrets'
BODY='{"request_id":"machine-01-20260927-000001","secrets":["账号001:密码001","账号002:密码002"],"note":"daily import"}'
BODY_MD5="$(printf '%s' "$BODY" | md5sum | awk '{print $1}')"
SIGN_STRING="POST\n${PATH}\n${TIMESTAMP}\n${BODY_MD5}"
SIGNATURE="$(printf '%s' "$SIGN_STRING" | openssl dgst -sha256 -hmac "$API_SECRET" -binary | od -An -tx1 | tr -d ' \n')"

curl -X POST 'https://YOUR-DOMAIN/api/v1/card-supply/secrets' \
  -H 'Content-Type: application/json' \
  -H "Dujiao-Next-Api-Key: ${API_KEY}" \
  -H "Dujiao-Next-Timestamp: ${TIMESTAMP}" \
  -H "Dujiao-Next-Signature: ${SIGNATURE}" \
  --data-binary "$BODY"
```

生产环境请使用 HTTPS，并避免在 shell history、CI 输出和请求日志中打印 API Secret。

## 7. 成功响应

HTTP 状态码为 `200`，响应结构如下：

```json
{
  "status_code": 0,
  "msg": "success",
  "data": {
    "request_id": "machine-01-20260927-000001",
    "batch_id": 123,
    "batch_no": "SUPPLY-20260927123000.000000000",
    "product_id": 10,
    "sku_id": 101,
    "received": 2,
    "created": 2,
    "skipped_duplicate": 0,
    "replayed": false
  }
}
```

字段说明：

- `received`：本次请求接收到的卡密数量（首尾空白处理后）；
- `created`：实际新建并进入可用库存的数量；
- `skipped_duplicate`：请求内重复或库存中已存在而跳过的数量；
- `replayed`：是否是相同 `request_id` 和相同 body 的幂等重放。

新建卡密的状态为 `available`，来源标记为 `supply_api`，因此会自动进入现有库存和发货链路。

## 8. 幂等和重试

`request_id` 是供号机必须维护的幂等键，作用域为当前 API source：

- 第一次请求成功后，如果没有收到响应，可以使用**相同的 `request_id` 和完全相同的 body** 重试；
- 相同 `request_id` + 相同 body 会返回原批次结果，响应中的 `replayed` 为 `true`，不会重复入库；
- 相同 `request_id` + 不同 body 会返回 `request_id_conflict`，不要继续复用这个 ID；
- 网络超时、连接断开等情况下建议指数退避重试；
- 收到 `batch_too_large`、`validation_error` 或 `request_id_conflict` 时不要盲目重试，应修正请求后使用新的 `request_id`（幂等重试除外）。

注意：卡密去重和 request ID 幂等是两层保护。即使换了新的 `request_id`，库存中已经存在的相同卡密也不会再次创建，但会计入 `skipped_duplicate`。

## 9. 错误响应

错误响应统一使用以下结构，并使用真实的 HTTP 状态码：

```json
{
  "status_code": 400,
  "msg": "request_id has already been used with a different body",
  "error_code": "request_id_conflict"
}
```

常见错误：

| HTTP | `error_code` | 说明 |
| ---: | --- | --- |
| 400 | `validation_error` | JSON 无效、字段缺失、卡密为空或包含换行 |
| 400 | `request_id_required` | 缺少 request ID |
| 401 | `supply_unauthorized` | 缺少/错误的 API Key、时间戳或签名 |
| 403 | `supply_source_disabled` | source 已禁用 |
| 403 | `ip_not_allowed` | 客户端 IP 不在 source 的白名单中 |
| 409 | `request_id_conflict` | request ID 已被不同 body 使用 |
| 413 | `body_too_large` | 原始请求体超过 2 MiB |
| 413 | `batch_too_large` | 卡密数量超过该 source 的批量上限 |
| 500 | `internal_error` | 服务端内部错误；可使用相同 request ID 和 body 重试 |

鉴权失败统一使用 `supply_unauthorized`，不会通过响应区分 API Key 是否存在。

## 10. IP 白名单

后台可以为 source 配置 IP 白名单，多个 IP 或 CIDR 可以使用逗号、空格或换行分隔，例如：

```text
203.0.113.10
203.0.113.0/24
```

配置白名单后，供号机必须从允许的公网出口 IP 发起请求。若供号机位于 NAT、代理或容器网络后，应填写服务端实际识别到的客户端出口 IP，并先用测试 source 验证。

## 11. Secret 重置

在后台点击 **重置 API Secret** 后：

1. 旧 API Secret 立即失效；
2. 页面只显示新的 API Secret 一次；
3. API Key 保持不变；
4. 供号机更新 Secret 后即可继续调用。

建议在确认供号机已切换到新 Secret 后，再停止旧配置的重试任务。
