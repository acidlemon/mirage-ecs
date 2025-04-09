#!/bin/bash

# mirage-ecsにAPIでタスクを立てるための簡素なシェルスクリプト
# JSONフォーマットでパラメータを渡せるようにしています

# デフォルト値
API_URL="http://mirage.dev.xuan-project.team/api/launch"
JSON_FILE=""
JSON_DATA=""

# 使い方を表示する関数
usage() {
  echo "使い方: $0 [-u API_URL] [-f JSON_FILE] [JSON_DATA]"
  echo ""
  echo "パラメータ:"
  echo "  JSON_DATA      JSONフォーマットのデータ (JSONファイルを指定しない場合は必須)"
  echo "  -f JSON_FILE   JSONデータを含むファイルのパス"
  echo "  -u API_URL     APIのURL (デフォルト: $API_URL)"
  echo "  -h             ヘルプを表示"
  echo ""
  echo "環境変数:"
  echo "  MIRAGE_API_URL APIのURL"
  echo ""
  echo "例:"
  echo "  $0 '{\"subdomain\":\"test\",\"branch\":\"master\",\"taskdef\":[\"default-task\"]}'"
  echo "  $0 -f params.json"
  echo "  $0 -u http://custom.mirage.com/api/launch -f params.json"
  exit 1
}

# 引数の解析
while getopts "u:f:h" opt; do
  case $opt in
    u) API_URL="$OPTARG" ;;
    f) JSON_FILE="$OPTARG" ;;
    h) usage ;;
    *) usage ;;
  esac
done

# 環境変数からの上書き
if [ -n "$MIRAGE_API_URL" ]; then
  API_URL="$MIRAGE_API_URL"
fi

# 残りの引数を取得
shift $((OPTIND-1))
if [ -z "$JSON_FILE" ]; then
  JSON_DATA="$1"
fi

# JSONデータの取得
if [ -n "$JSON_FILE" ]; then
  # ファイルが存在するか確認
  if [ ! -f "$JSON_FILE" ]; then
    echo "エラー: JSONファイル '$JSON_FILE' が見つかりません"
    exit 1
  fi

  # ファイルからJSONデータを読み込む
  JSON_DATA=$(cat "$JSON_FILE")
  if [ $? -ne 0 ]; then
    echo "エラー: JSONファイルの読み込みに失敗しました"
    exit 1
  fi

  echo "JSONファイル '$JSON_FILE' からデータを読み込みました"
elif [ -z "$JSON_DATA" ]; then
  echo "エラー: JSONデータまたはJSONファイルを指定してください"
  usage
fi

# curlコマンドのオプション
CURL_OPTS="-s -X POST -H \"Content-Type: application/json\""

# APIリクエストの実行
echo "APIリクエストを実行しています..."
echo "URL: $API_URL"
echo "データ: $JSON_DATA"

CURL_CMD="curl $CURL_OPTS -d '$JSON_DATA' $API_URL"
echo "実行コマンド: $CURL_CMD"
echo ""

# evalを使用してコマンドを実行
RESPONSE=$(eval $CURL_CMD)
STATUS=$?

if [ $STATUS -ne 0 ]; then
  echo "エラー: APIリクエストが失敗しました (ステータスコード: $STATUS)"
  exit 1
fi

echo "レスポンス:"
echo $RESPONSE | jq . 2>/dev/null || echo $RESPONSE

# レスポンスの結果を確認
if echo $RESPONSE | grep -q '"result":"ok"'; then
  echo "タスクの起動に成功しました"
  exit 0
else
  echo "タスクの起動に失敗しました"
  exit 1
fi
