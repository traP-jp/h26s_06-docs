## Tools

### ローカル実行

```bash
bun dev
```

### モックサーバー

- `?demo=1` を付与してアクセスすると、traQ に接続せず、テスト用のデータを利用できる。

####　起動

```bash
bun mock:up
```

#### 停止

```bash
bun mock:down
```

### Cloud Run

production image は `$PORT` (default `5173`) で `dist` を配信する。
`GCLOUD_BACKEND_PROXY=true` の場合のみ、`/api` を `SERVER_UPSTREAM`
(default `http://localhost:8080`) に転送する。

Cloud Run で client container を ingress にして server container を sidecar にする場合は、
client container に `GCLOUD_BACKEND_PROXY=true` と
`SERVER_UPSTREAM=http://localhost:8080` を設定する。
client / server を別々の Cloud Run service にする場合は、server service の URL を
`SERVER_UPSTREAM` に設定する。
