# BINANCE — TrimUI Brick Pro (Stock OS)

Ứng dụng **chỉ xem dữ liệu Binance Spot** trên TrimUI Brick Pro. Không đăng nhập tài khoản, không giao dịch.

## Tự build bằng GitHub Actions

1. **Một lần duy nhất:** tải `BinanceGia_Bootstrap_Source_v0.29.zip` do ChatGPT cung cấp vào **thư mục gốc** repository qua **Add file → Upload files → Commit changes**. Đừng giải nén trước khi upload và đừng đổi tên file.
2. Workflow [`00 - Import initial Binance source`](.github/workflows/00-import-source.yml) sẽ kiểm tra ZIP, giải nén mã nguồn vào `src/`, file chạy và tài nguyên vào `app/`, chạy test và build, sau đó commit source và xóa ZIP bootstrap.
3. Từ đây, khi ChatGPT sửa `src/` hoặc `app/`, workflow [`01 - Build and test`](.github/workflows/01-build.yml) sẽ **tự kiểm thử và tạo ZIP**, nhưng **KHÔNG phát hành OTA**.
4. Khi bạn đã duyệt bản build và muốn cập nhật lên máy, mở [Actions](../../actions), chọn **02 - Publish OTA (manual approval)** → **Run workflow**; nhập tag đúng với `appVersion` trong `src/main.go` (ví dụ `v0.29`) và xác nhận `PUBLISH`.
5. Workflow phát hành chạy lại `go test`, `go vet`, cross-build **Linux ARM64 static**, đóng gói `Apps/BinanceGia.pak/`, tạo GitHub Release và cập nhật `manifest.json` **sau khi ZIP đã được đăng**.

Đây là chủ ý an toàn: **commit mã nguồn không tự đẩy bản cập nhật lên TrimUI**. Chỉ thao tác publish được xác nhận mới thay đổi `manifest.json` OTA.

## Cập nhật OTA trên thiết bị

- URL manifest: `https://raw.githubusercontent.com/phongdinh91/trimui-binance-ota/main/manifest.json`
- Tên Release ZIP: `BinanceGia_TrimUI_StockOS_vX.YY.zip`.
- Bản khởi động OTA (để thử `v0.28` → `v0.29`) là gói riêng `BinanceGia_OTA_Bootstrap_v0.28.zip`.
- App kiểm tra version, tải ZIP qua HTTPS, kiểm tra SHA-256 và giữ nguyên `favorites.json`, `settings.json`, cache, `ota.json` và log.

**Lưu ý:** manifest chỉ nên trỏ tới bản đã được kiểm thử. Tệp `manifest.json` được sinh tự động — không sửa trước khi Release tồn tại.

## Cấu trúc repository

```text
src/main.go, src/main_test.go, src/go.mod     # mã nguồn Go
app/launch.sh, app/config.json, app/ota.json  # launcher và cấu hình
app/icon.png, app/cacert.pem, app/assets/     # giao diện và chứng chỉ
scripts/package.sh                            # đóng gói Stock OS
.github/workflows/00-import-source.yml        # nhập nguồn lần đầu
.github/workflows/01-build.yml                # tự build, không phát hành
.github/workflows/02-publish.yml              # phát hành khi bạn phê duyệt
.github/workflows/publish-ota.yml             # hỗ trợ Release phát hành thủ công
```

Nếu workflow báo lỗi `Resource not accessible by integration` hoặc không đẩy được commit, kiểm tra **Settings → Actions → General → Workflow permissions → Read and write permissions**.

## Các phiên bản sau

ChatGPT chỉnh sửa trực tiếp mã nguồn trên GitHub (hoặc qua pull request). Push code → GitHub tự test và tạo bản thử. Bạn chủ động bấm Publish sau khi hài lòng. Không cần tải ZIP thủ công nữa.
