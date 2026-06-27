# フロントエンド コアロジック・データストア設計

## 1. 全体構造

フロントエンドのデータは、次の3つの役割に分けて管理する。

### ChannelGraph

純粋な TypeScript クラスとして実装するロジック層である。7,000ノードの親子関係、スコア計算、波紋の連鎖ロジックなど、毎フレームの重い計算を担当する。Vue のリアクティブシステムからは完全に切り離す。

### NodeBuffer

`Float32Array` を使うバッファ層である。ChannelGraph で計算した結果を GPU（Three.js）へ転送するための一次元配列として扱う。座標、サイズ、色などの描画用データを保持する。

### AppState

Vue の `ref` や `reactive` で管理する UI 層である。現在ユーザーがクリックして選択しているチャンネルの ID や名前、全体のローディング状態など、UI の描画に必要な最小限のデータだけを持つ。

## 2. インターフェース設計

各ノード（チャンネル）が持つべき論理データを定義する。

```ts
interface ChannelNode {
  index: number;         // 0~6999 の固定インデックス（配列アクセス用・最速）
  id: string;            // UUID（API との照合用）
  parentId: string | null;
  children: number[];    // 子ノードのインデックス配列

  islandId: number;      // 属する島の ID（0~8）
  depth: number;         // ルートからの深さ（色や配置の計算に使用）

  // 状態（スコア）
  currentScore: number;   // 現在の盛り上がり度（サーバーと同じ raw な相対値）
  targetScore: number;    // サーバーから同期された目標スコア（Lerp 用）
  relativeScore: number;  // 描画に使う 0.0~1.0 の相対値
}
```

## 3. クラス設計

### ChannelGraph

7,000件のデータを配列で保持し、毎フレームの計算を超高速に行うクラスである。

```ts
class ChannelGraph {
  nodes: ChannelNode[] = [];
  nodeMap: Map<string, number> = new Map(); // ID -> index の高速ルックアップ用

  constructor(initData: any[]) {
    // サーバーからの初期データ（init）をパースし、7000件の nodes を構築する
  }

  // イベント処理

  // 投稿インパルスの受信
  triggerPostEvent(targetId: string) {
    const index = this.nodeMap.get(targetId);
    if (index === undefined) return;

    // 対象ノードのスコアをスパイクさせる
    this.nodes[index].currentScore += 1.0;

    // 親への「波紋連鎖タイマー」をイベントキューに登録する
    this.scheduleRippleToParent(this.nodes[index]);
  }

  // サーバーからの定期同期（activity_update）の受信
  syncServerScores(updates: { id: string; score: number }[]) {
    for (const update of updates) {
      const index = this.nodeMap.get(update.id);
      if (index !== undefined) {
        // 現在値は上書きせず、目標値（targetScore）のみを更新する
        this.nodes[index].targetScore = update.score;
      }
    }
  }
}
```

### NodeBuffer

GPU へ転送するデータを管理する。Three.js の `InstancedMesh` は、位置とサイズを `Matrix4`（16要素）、色を `Color`（3要素）で受け取る。

```ts
class NodeBuffer {
  // 1インスタンスあたり 16個の Float 値（Matrix4）
  matrixData: Float32Array;
  // 1インスタンスあたり 3個の Float 値（R, G, B）
  colorData: Float32Array;

  constructor(count: number) {
    this.matrixData = new Float32Array(count * 16);
    this.colorData = new Float32Array(count * 3);
  }

  // 初期の3D座標をセットする（静的配置タスク 2.1 用）
  setPosition(index: number, x: number, y: number, z: number) {
    // matrixData の該当インデックスに平行移動をセットする
  }
}
```

## 4. メインループ

`requestAnimationFrame` 内で毎フレーム実行される、すべての層をつなぐ心臓部である。

```ts
function updateFrame(deltaTime: number) {
  // 1. ロジック層: 全ノードの計算（0 から 6999 までフラットな配列を回すのが最速）
  for (let i = 0; i < channelGraph.nodes.length; i++) {
    const node = channelGraph.nodes[i];

    // a) 自律減衰（Exponential Decay）
    // 時定数約300秒で緩やかに減らす
    node.currentScore *= Math.exp(-deltaTime / 300);

    // b) サーバー値への補間（Lerp）
    // 目標値との差分を少しずつ埋める（例: 差分の10%を近づける）
    node.currentScore += (node.targetScore - node.currentScore) * 0.1;

    // 2. バッファ層への書き込み
    // 全ノードの最大付近を基準にした relativeScore からスケールや輝度を計算する
    const scale = calculateScaleFromScore(node.relativeScore);
    const brightness = calculateBrightness(node.relativeScore);

    // バッファの更新（matrixData と colorData を書き換える）
    nodeBuffer.updateInstance(i, scale, brightness);
  }

  // 3. GPU 層（Three.js）への転送フラグを立てる
  instancedMesh.instanceMatrix.needsUpdate = true;
  instancedMesh.instanceColor.needsUpdate = true;
}
```
