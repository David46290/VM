# Go 入門筆記：從 Python 轉過來

這份筆記整理本次學習時遇到的問題，聚焦在 Go 與 Python 思考方式的差異。

## 專案與 Go module

### 為什麼 debugger 說找不到 main module？

`go run main.go` 可以直接指定單一檔案執行；VS Code 的 Go debugger 通常會建置整個資料夾（例如 `.`）。Go 需要用 `go.mod` 知道目前資料夾屬於哪個 module，因此在專案根目錄初始化一次：

```powershell
go mod init example.com/vm
```

這會在本機建立 `go.mod`，並不會連到 GitHub。之後即使程式仍只在本機，也可以正常建置和除錯。

`github.com/David46290/VM` 這種 module path 是專案的識別名稱，常用於發布或被其他 Go 專案引用；它不是檔案路徑，也不代表 Go 會去 GitHub 讀取程式。若未發布，也可使用 `example.com/vm` 等名稱。

### VS Code 如何除錯 Go？

安裝 Microsoft Go 擴充套件後，在行號左側設中斷點並按 F5。除錯器會先編譯本機專案，再由 Delve 停在中斷點。可在 Variables 面板或滑鼠懸停查看變數；程式中也能用 `fmt.Printf("%T\\n", value)` 印出型態。

### VS Code 按 Tab 沒有縮排

常見原因是 Tab 被設定成移動焦點。按 `Ctrl+M` 切換 **Tab Key Moves Focus**，或從命令面板執行 **Toggle Tab Key Moves Focus**。確認焦點在程式碼編輯器，右下角語言模式是 Go。Go 格式化通常會使用 Tab 字元。

## 讀取資料夾中的圖片

### `os.ReadDir` 回傳的是什麼？

`os.ReadDir(dir)` 回傳該資料夾內的 `os.DirEntry` 清單。`DirEntry` 是目錄項目資訊，不是已開啟的檔案，也不包含父資料夾路徑：

```go
entries, err := os.ReadDir(targetDir)
if err != nil {
	return err
}

for _, entry := range entries {
	if entry.IsDir() {
		continue
	}

	name := entry.Name()
	path := filepath.Join(targetDir, name)
	// path 可交給 os.Open
}
```

`entry.Name()` 取得檔名；`filepath.Join(targetDir, entry.Name())` 組成完整路徑。這是 `ReadDir` 的正常用法。如果使用 `filepath.WalkDir` 遞迴走訪，callback 會直接收到每個項目的路徑。

### 如何用標準函式庫解碼圖片？

Go 標準函式庫的 `image` 提供通用圖片介面；格式解碼器分開放在 `image/jpeg`、`image/png`、`image/gif`。要讓通用的 `image.Decode` 自動辨認格式，可只為初始化註冊而空白匯入：

```go
import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)
```

`_` 表示載入該套件並執行初始化，但不在程式中直接使用套件名稱。JPEG/PNG/GIF 套件會在初始化時註冊各自的解碼器；`image.Decode` 再依檔案內容選擇解碼器。若直接匯入但沒有使用套件名稱，Go 會報未使用的匯入。

若只處理單一格式，也可正常匯入該格式套件並直接呼叫它的解碼函式。通用解碼方式則適合讓多種格式共用同一段程式。

### 建議的單檔驗證函式

```go
func validateFile(path string) (image.Image, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}
```

開檔失敗或解碼失敗都回傳 `error`。呼叫端可以依需求跳過非圖片檔、記錄錯誤或中止處理。成功時可用 `img.Bounds()` 取得圖片範圍，例如寬高為 `img.Bounds().Dx()` 和 `img.Bounds().Dy()`。

## `defer` 與資源清理

`defer` 登記一個函式呼叫，等目前函式返回時執行。多個 defer 會以後進先出（LIFO）順序執行。呼叫參數通常在 defer 那一刻求值，但被呼叫的函式是在返回時才執行。

```go
file, err := os.Open(path)
if err != nil {
	return err
}
defer file.Close()
```

這能確保無論後續從哪個 `return` 離開，都會關閉檔案。若只在函式尾端手動 `Close`，每個提早返回路徑都要各自記得關閉，容易漏掉。

`defer` 的範圍是所在的函式：放在 `main` 的迴圈內，檔案會等 `main` 返回才關閉；放在每次處理單一檔案的子函式內，則每次子函式返回就關閉當次檔案。因此把開啟、解碼、關閉放在 `validateFile` 這類函式裡很合適。

## 為什麼 Go 常回傳 `error`？

Go 把可預期、呼叫端可能處理的失敗當成一般回傳值。呼叫端必須明確檢查並決定如何處理：

```go
img, format, err := validateFile(path)
if err != nil {
	fmt.Printf("跳過 %s：%v\\n", path, err)
	continue
}
```

這比例外自動跳出多層呼叫更明確：從程式碼能看出哪個操作會失敗，以及呼叫端採取什麼處置。一般錯誤用 `error` 回傳；`panic` 通常留給程式無法合理繼續的嚴重狀況。

回傳 `(image.Image, string, error)` 通常比 `(bool, image.Image, string)` 清楚，因為 `error` 可以帶有失敗原因；成功時 `err == nil`，失敗時回傳 `nil` 圖片和空格式字串。

## Git 與 GitHub 補充

- `git config --global user.name` 和 `user.email` 設定的是 commit 作者資訊，不是 GitHub 密碼。
- GitHub 推送驗證使用瀏覽器登入或 Git Credential Manager；不要把密碼、token 放進 Git 設定或貼在聊天中。
- 若 PowerShell 顯示 `git` 不是命令，代表 Git 未安裝或未加入 PATH。安裝 Git for Windows 後重新開啟 VS Code，再用 `git --version` 確認。
- 上傳前要檢查是否包含密碼、token 或其他不應公開的檔案；公開 repo 會讓任何人都能看到內容。
- Copilot 帳號的個人用量無法從本機專案或聊天中查出；應登入 GitHub 帳戶查看方案與用量頁面。模型 context window 是單次對話容量，不等同帳號的月度用量額度。

## 圖片型別、色彩分量與通道

### `image.Image` 與 `ColorModel()` 各代表什麼？

`image.Image` 是介面，定義所有圖片都能提供的基本操作，例如 `Bounds()`、`At(x, y)`、`ColorModel()`。它不公開每種圖片的底層儲存方式；JPEG 解碼後可能得到 `*image.YCbCr`，其他格式或解碼器也可能使用不同實作。

`ColorModel()` 回傳 `color.Model`，用途是把顏色轉換成某種表示方式，不是描述影像有幾個通道或列出底層像素資料。方法不會像 struct 欄位一樣在 debugger 展開時出現。要知道實際型別可用：

```go
fmt.Printf("image type: %T\n", img)
```

### Y、Cb、Cr 是什麼？

YCbCr 有三個色彩分量：

- `Y`：亮度分量。
- `Cb`：藍色差異分量。
- `Cr`：紅色差異分量。

Go 的 `*image.YCbCr` 以 `Y`、`Cb`、`Cr` 三個 `[]uint8` 切片儲存樣本。`Y` 通常每個像素位置有一個亮度樣本；`Cb`、`Cr` 可能因色度子取樣而少很多。例如 4:2:0 中，一個色度樣本大致由 2×2 個亮度位置共用。用 `SubsampleRatio` 查看子取樣方式。

概念上 YCbCr 有三個分量，但切片長度還會受到影像矩形範圍、每列 stride 等儲存細節影響，不能單靠 `len` 推算通道數或每像素資料大小。

### 「通道數」要先說明指哪一種

通道數會依問題的語境不同而有不同意思：色彩模型的分量數、底層影像的儲存平面／樣本，或像素取樣後提供的顏色值。`image.Image` 是抽象介面，因此不存在一個對所有實作都通用、能由介面直接取得的儲存通道數。

例如 YCbCr 有 Y/Cb/Cr 三個分量，但 Cb、Cr 可以子取樣；`image.RGBA` 有 R/G/B/A 四個儲存分量；灰階圖片通常只有亮度值；調色盤圖片可能儲存索引，再透過 palette 對應顏色。因此「RGBA 是四個分量」不等於每種情境下都應把圖片描述成四個同等意義的色彩通道，alpha 是透明度資訊。

若目標是取得像素呈現的顏色，不必知道底層如何儲存，可以呼叫：

```go
r, g, b, a := img.At(x, y).RGBA()
```

它回傳標準化的 RGBA 數值（各為 16-bit 範圍），不表示原始圖片一定以 RGBA 四通道格式儲存。

### 用 type switch 檢查具體圖片型別

介面值可以裝入不同的具體型別；type switch 會依執行時裝入的型別分支：

```go
switch typedImg := img.(type) {
case *image.YCbCr:
	fmt.Println(typedImg.SubsampleRatio)
case *image.RGBA:
	fmt.Println("RGBA representation")
case *image.Gray:
	fmt.Println("grayscale representation")
default:
	fmt.Printf("other representation: %T\n", typedImg)
}
```

在每個 `case` 裡，`typedImg` 會是該 case 指定的型別。`image.YCbCr` 是 struct 型別；`*image.YCbCr` 是指向該 struct 的指標，不是路徑。兩者是不同型別。解碼器常回傳指標型別，所以要用 `%T` 或 type switch 確認實際型別。Go 會自動處理指標 struct 的欄位存取，例如 `typedImg.SubsampleRatio`。

### 在 debugger 裡檢視方法和結果

VS Code 的 Go debugger 使用 Delve。變數展開主要顯示欄位，不會列出方法；`Bounds()`、`ColorModel()` 是方法，不是圖片 struct 裡的資料欄位。Debug Console 直接求值方法呼叫可能被拒絕（例如提示需使用 `call`），而且方法呼叫支援受 debugger 限制。

較穩定的檢查方式是在程式碼中先保存結果，再於下一行設中斷點：

```go
bounds := img.Bounds()
colorModel := img.ColorModel()
```

接著在 Variables 面板查看 `bounds` 或 `colorModel` 的實際值。若要知道可用方法或型別定義，可對型別按 F12，使用 IntelliSense，或查文件：

```powershell
go doc image.Image
go doc image.YCbCr
go doc image.Rectangle
```

通常先看介面／變數型別，再檢查具體型別或方法的回傳型別即可，不需要從標準函式庫逐個型別盲查。
