# footcd

`cd` は通常どおり動かしつつ、引数が `-` のときだけ、過去に `cd` したディレクトリ履歴から移動先を選べるようにする実装です。配布物は 1 バイナリだけで動きます。

## 仕組み

外部コマンド単体では親シェルのカレントディレクトリを変更できないため、バイナリ自身が初期化用シェルコードを出力し、それを `eval` して `cd` 関数を定義します。

- Go バイナリ `footcd`: 履歴の選択、保存、初期化コード出力を行う
- シェル関数 `cd`: `footcd init bash` などの出力を `eval` して定義する

## ビルド

```bash
make build
```

主要 OS / ARCH 向けのクロスビルド:

```bash
make cross
```

出力先:

- `footcd`
- `dist/footcd-linux-amd64`
- `dist/footcd-linux-arm64`
- `dist/footcd-darwin-amd64`
- `dist/footcd-darwin-arm64`
- `dist/footcd-windows-amd64.exe`
- `dist/footcd-windows-arm64.exe`

## Bash / Zsh への組み込み

```bash
eval "$(footcd init bash)"
```

Zsh なら以下です。

```bash
eval "$(footcd init zsh)"
```

これで `cd` 関数が上書きされ、通常時は `builtin cd` と同じように動作し、`cd -` のときだけ履歴選択になります。

永続化するなら `~/.bashrc` や `~/.zshrc` に書いてください。

## 使い方

通常の移動:

```bash
cd src
```

履歴から選択:

```bash
cd -
```

`cd -` を実行すると履歴一覧が表示され、番号を入力するとそのディレクトリへ移動します。

初期化コードだけ見たい場合:

```bash
footcd init bash
```

## 履歴

- デフォルト保存先: `$XDG_CONFIG_HOME/footcd/history` または `~/.config/footcd/history`
- 環境変数 `FOOTCD_HISTORY_FILE` で変更可能
- 環境変数 `FOOTCD_HISTORY_LIMIT` で件数上限を変更可能

履歴には実際に移動した絶対パスだけを保存し、存在しなくなったディレクトリは候補から除外します。
