# BINANCE — TrimUI Brick Pro (Stock OS)

Ứng dụng chỉ xem dữ liệu Binance Spot, nến và chỉ báo MA/BOLL/MACD. **Không giao dịch hoặc đặt lệnh.**

## Tự động phát hành OTA

- **[Build & test ARM64](.github/workflows/build.yml):** mỗi thay đổi mã nguồn (`src/`) hoặc tài nguyên (`app/`) được kiểm thử bằng `go test ./...`, `go vet ./...`, biên dịch Linux ARM64 và đóng gói ZIP kiểm thử dưới **Artifacts**. Không phát hành OTA chỉ vì sửa code.
- **[Auto publish Binance OTA on version bump](.github/workflows/release.yml):** khi commit lên `main` làm tăng `appVersion` trong `src/main.go`, GitHub tự chạy kiểm thử, biên dịch, đóng gói, tải ZIP lên **GitHub Releases**, xác minh file đã tải, sau đó cập nhật `manifest.json` và SHA-256. **Không cần bấm Run workflow hay PUBLISH.**
- Nếu phiên bản không tăng, workflow tự bỏ qua việc phát hành. Nếu build/test lỗi, không tạo Release mới.
- Nếu Release đã tồn tại, workflow không ghi đè ZIP đã phát hành.

### Cách ra bản mới bằng ChatGPT

1. Yêu cầu ChatGPT thêm hoặc sửa tính năng; mã nguồn và ảnh được cập nhật trong repo.
2. Sau khi hoàn thiện thay đổi, tăng `appVersion` trong `src/main.go`, ví dụ `v0.29` → `v0.30`, rồi commit lên nhánh `main`.
3. Xem kết quả trong [GitHub Actions](https://github.com/phongdinh91/trimui-binance-ota/actions).
4. Nếu workflow phát hành thành công, bản ZIP có tại [Releases](https://github.com/phongdinh91/trimui-binance-ota/releases) và `manifest.json` sẽ trỏ đến ZIP mới.

**Lưu ý an toàn:** Build và unit test không thay thế kiểm thử trên Brick Pro thật. Bật tự động phát hành nghĩa là một lần tăng version có thể gửi OTA tới người dùng trước khi kiểm thử trên thiết bị. Thiết bị vẫn phải hỏi người dùng xác nhận trước khi cài.

### Phiên bản hiện tại

Mã nguồn đang ở `v0.29`. Việc bật workflow hôm nay **không tự phát hành lại phiên bản đã có sẵn**. Workflow tự phát hành bắt đầu từ lần tiếp theo tăng `appVersion` (chẳng hạn `v0.30`). Nếu muốn phát hành `v0.29` ngay cần một bước phát hành khởi tạo riêng.

## Cấu hình thiết bị

Địa chỉ bản kê OTA:

`https://raw.githubusercontent.com/phongdinh91/trimui-binance-ota/main/manifest.json`

Lần đầu phải cài gói Bootstrap có `Apps/BinanceGia.pak/ota.json` trỏ tới URL này, đồng thời kết nối Wi-Fi. App hỏi trước khi nâng cấp và phải giữ dữ liệu `favorites.json`, `settings.json`, `market-cache.json` và nhật ký.

## Cấu trúc repo

- `src/`: mã nguồn và kiểm thử Go.
- `app/`: giao diện, icon, launcher, cấu hình OTA.
- `.github/workflows/build.yml`: build thử khi có thay đổi.
- `.github/workflows/release.yml`: phát hành tự động chỉ khi tăng version.
- `scripts/package.sh`: script đóng gói phụ trợ.
