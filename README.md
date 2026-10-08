# BINANCE cho TrimUI Brick Pro — Stock OS

Ứng dụng chỉ xem giá Binance Spot, biểu đồ nến và chỉ báo MA/BOLL/MACD. **Không hỗ trợ giao dịch.**

## Quy trình phát triển bằng ChatGPT + GitHub Actions

Repository dùng **hai workflow chính**:

1. [Build & test ARM64](.github/workflows/build.yml): tự động khi `src/` hoặc `app/` thay đổi; chạy `go test`, `go vet`, build Linux ARM64 tĩnh, đóng gói `Apps/BinanceGia.pak/` và lưu bản thử nghiệm trong **Artifacts**. Workflow này **không phát hành OTA**.
2. [Publish approved Binance OTA](.github/workflows/release.yml): chỉ chạy khi bạn tự bấm **Run workflow**, điền version và xác nhận **PUBLISH**. Sau khi kiểm thử, workflow tạo ZIP ARM64, tạo GitHub Release, tính SHA-256 và cập nhật `manifest.json` trên nhánh `main`.

## Hướng dẫn phát hành v0.29

1. Vào [Actions — Build & test ARM64](https://github.com/phongdinh91/trimui-binance-ota/actions/workflows/build.yml), bảo đảm lần build mới nhất **Success**.
2. Nếu muốn thử trước, tải `binance-arm64-test-build` từ phần **Artifacts**, giải nén vào thẻ nhớ và kiểm tra trực tiếp trên Brick Pro.
3. Mở [Publish approved Binance OTA](https://github.com/phongdinh91/trimui-binance-ota/actions/workflows/release.yml) → **Run workflow**.
4. Điền `v0.29` ở trường **version**, chọn **PUBLISH**, rồi xác nhận chạy.
5. Đợi workflow xanh và xem [Releases](https://github.com/phongdinh91/trimui-binance-ota/releases). File `manifest.json` sẽ trỏ tới ZIP vừa phát hành kèm checksum SHA-256.

**Quan trọng:** Chỉ xác nhận PUBLISH sau khi đã thử bản build trên máy. Không nên phát hành OTA bản chưa kiểm thử vì có thể làm ứng dụng không chạy.

## Thiết bị và OTA

Ứng dụng kiểm tra cập nhật qua URL:

`https://raw.githubusercontent.com/phongdinh91/trimui-binance-ota/main/manifest.json`

Lần đầu cần cài bản Binance có `ota.json` trỏ tới URL trên (gói bootstrap v0.28 đã chuẩn bị trong ChatGPT). Khi có Wi-Fi và bản mới, app sẽ hỏi trước khi cập nhật. `favorites.json`, `settings.json`, `market-cache.json` và nhật ký **không nằm trong gói phát hành**, nên không bị đóng gói ghi đè.

## Cấu trúc

- `src/main.go`, `src/main_test.go`, `src/go.mod`: mã nguồn Go và kiểm thử.
- `app/`: icon, ảnh giao diện, launcher, CA certificate được đưa vào ZIP trong workflow, cấu hình OTA.
- `.github/workflows/build.yml`: tự động kiểm thử và build.
- `.github/workflows/release.yml`: phát hành OTA với xác nhận PUBLISH.
- `scripts/package.sh`: script đóng gói phụ trợ.

Lịch sử những lần chạy workflow cũ vẫn có thể xuất hiện trong tab Actions, nhưng các workflow cũ trùng chức năng đã được loại bỏ khỏi repo.
