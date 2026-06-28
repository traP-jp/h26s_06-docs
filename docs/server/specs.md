# バックエンド詳細仕様書

## 1. システムの基本方針

本システムは、traQ上のリアルタイムなコミュニケーション・アクティビティを視覚化するためのバックエンド基盤である。

* **役割の明確化**: バックエンドは「イベントの集約」と「最小限のインパルス送信」「定期的な状態の正解同期」に特化する。波紋の伝播計算や時間経過による滑らかな減衰などの重い処理はすべてフロントエンドに委譲する。
* **非機能要件の遵守**: 7000チャンネル・最大700名同時接続という環境下において、メモリ使用量を常に150MB以下に抑え、ネットワーク帯域を最小化する設計を徹底する。
* **通信プロトコル**: リアルタイムかつ単方向のデータストリームに最適なSSE (Server-Sent Events) を採用する。

---

## 2. インメモリデータ構造と状態管理

高速なアクセスとメモリ効率を両立するため、システム全体のチャンネルは「Grand Root」を頂点とする単一のフラットなマップとして管理する。

### 2.1 データ構造 (Go Struct)

ポインタによるツリー構造はGCの負荷を上げるため避け、ID（文字列）をキーとしたフラットな辞書型（`map`）で状態を保持する。定期同期のためのメタデータもここに含める。

```go
// システム全体の根となる仮想ID
const GrandRootID = "grand_root"

// Channel: 各チャンネルの静的情報と動的スコアを管理
type Channel struct {
	ID            string
	ParentID      string   // 最上位ノードは GrandRootID を指す
	Children      []string // 子チャンネルIDのリスト
	Score         float64  // バックエンド側が持つ相対的な「盛り上がり度」
	LastSyncScore float64  // 前回フロントへ送信した時点のスコア
	LastSyncTime  time.Time// 前回フロントへ送信した時刻
}

// UserState: ユーザーの現在位置を管理（ビームのアニメーション計算用）
type UserState struct {
	UserID         string
	CurrentChannel string    // 現在閲覧中のチャンネルID
	LastUpdated    time.Time // 最終更新日時
}

// StateManager: システム全体の状態をスレッドセーフに管理
type StateManager struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	users    map[string]*UserState
}

```

### 2.2 排他制御 (Race Condition対策)

`StateManager` 全体に単一の `sync.RWMutex` を配置する。

* **Lock (書き込み)**: 投稿イベント受信時、スコア加算時、ユーザーのチャンネル移動検知時。
* **RLock (読み込み)**: フロントへの差分データ（syncイベント）生成時、ツリー構造探索時。

---

## 3. コアロジックの詳細

### 3.1 初期化データのキューイング（メモリスパイク対策）

150MBのメモリ制約を守るため、700人が一斉に接続してきた際のHTTP書き込みバッファのスパイクをセマフォで制御する。同時に巨大なJSONを送信できるゴルーチン数を制限する。

```go
const maxConcurrentInits = 10
var initSemaphore = make(chan struct{}, maxConcurrentInits)

func handleSSEConnect(w http.ResponseWriter, r *http.Request) {
	initSemaphore <- struct{}{}
	sendInitData(w) // キャッシュ済みJSONの送信
	w.(http.Flusher).Flush()
	<-initSemaphore
	
	// 以降、trigger / sync イベントの受信ループへ移行
}

```

### 3.2 移動検知ロジック

traQからのイベント受信時、インメモリの `users["userID"].CurrentChannel` と比較し、変化があれば `trigger(type: mov)` イベントを生成して `CurrentChannel` を更新する。

### 3.3 確率的な同期イベントの送信アルゴリズム

全チャンネルのスコアが常に減衰し続ける仕様において、差分抽出を効果的に機能させるため、**「スコア変化量」と「経過時間」に基づく重み付き同期**を行う。

30秒ごとのTicker処理において、各チャンネルの重み $W$ を以下の数式で算出する。算出した重みは全候補で正規化し、最大100件を重み付き抽選で `sync` ペイロードに含める。

$$W = \alpha \times |\Delta S| + \beta \times \Delta T$$

* $\Delta S$: 前回同期時からのスコア変化量 (`math.Abs(Score - LastSyncScore)`)
* $\Delta T$: 前回同期時からの経過時間（秒）
* $\alpha, \beta$: 抽選重みを調整する係数

`Score` は固定上限を持たない相対値として扱い、init および sync 配信時は小数第3位に丸める。投稿は対象チャンネルに `messageScoreAmount * log(1 + 文字数)`、閲覧移動は `0.25` を加算し、親チャンネルへは階層ごとに `0.45` 倍して伝播する。`trigger` SSE イベントの `delta` には対象チャンネル（深さ0）へ加算した値を入れる。減衰は指数減衰で、時定数は約300秒とする。

```go
func generateSyncPayload() {
	now := time.Now()
	deltas := make(map[string]float64)

	for _, ch := range sm.channels {
		deltaS := math.Abs(ch.Score - ch.LastSyncScore)
		deltaT := now.Sub(ch.LastSyncTime).Seconds()

		weight := (alpha * deltaS) + (beta * deltaT)

		weighted = append(weighted, WeightedChannel{ID: ch.ID, Weight: weight})
	}
	for _, ch := range selectWeightedChannels(weighted, 100) {
		deltas[ch.ID] = ch.Score
		ch.LastSyncScore = ch.Score
		ch.LastSyncTime = now
	}
	// deltas を送信キューへ投入
}

```
