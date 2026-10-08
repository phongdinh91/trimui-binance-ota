# BINANCE — TrimUI Brick Pro (Stock OS)

Ứng dụng chỉ đọc giá, biểu đồ nến và chỉ báo MA/BOLL/MACD từ Binance Spot. **Không có chức năng đặt lệnh.**

## Đã thiết lập GitHub Actions

- [Build & test ARM64](.github/workflows/build.yml): tự chạy khi mã nguồn hoặc tài nguyên thay đổi. Kiểm tra Go, build binary ARM64 tĩnh, đóng gói `Apps/BinanceGia.pak/` và lưu file ZIP ở mục **Artifacts**. **Không phát hành OTA** khi chỉ sửa code.
- [Publish approved Binance OTA](.github/workflows/release.yml): **chỉ chạy khi bạn chủ động chọn Run workflow**. Tự kiểm thử, build ZIP, tạo GitHub Release, tính SHA-256 và cập nhật `manifest.json`.
- [Publish OTA manifest](.github/workflows/publish-ota.yml): hỗ trợ tạo manifest khi GitHub Release được xuất bản thủ công.

## Phát hành v0.29 (hoặc phiên bản tiếp theo)

1. Truy cập [GitHub Actions](https://github.com/phongdinh91/trimui-binance-ota/actions).
2. Chọn **Build & test ARM64** và kiểm tra lần build mới nhất có dấu tích xanh; bạn có thể tải **binance-arm64-test-build** ở trang chi tiết workflow để thử trên máy.
3. Sau khi kiểm tra, chọn **Publish approved Binance OTA** → **Run workflow** → điền `v0.29` (hoặc phiên bản bạn muốn phát hành) → xác nhận.
4. Chờ workflow xanh; kiểm tra ZIP trong [Releases](https://github.com/phongdinh91/trimui-binance-ota/releases) và `manifest.json` ở nhánh `main`.

**Lưu ý:** Tuyệt đối không phát hành OTA nếu chưa kiểm tra gói build trên TrimUI Brick Pro. Cập nhật lỗi có thể khiến app không chạy trên máy.

## Thiết bị

Ứng dụng đọc manifest tại:

`https://raw.githubusercontent.com/phongdinh91/trimui-binance-ota/main/manifest.json`

Trên Brick Pro, cần cài lần đầu phiên bản có `Apps/BinanceGia.pak/ota.json` trỏ về URL này (gói OTA Bootstrap v0.28 đã chuẩn bị). Sau đó máy bật Wi-Fi sẽ kiểm tra cập nhật và hỏi trước khi cài. Danh sách yêu thích, cài đặt và cache không nằm trong gói cài.

## Cấu trúc repo

- `src/main.go`, `src/main_test.go`, `src/go.mod`: mã nguồn và kiểm thử.
- `app/`: icon, ảnh giao diện, cấu hình, launcher, địa chỉ OTA.
- `scripts/package.sh`: script đóng gói thay thế; tự đưa chứng chỉ CA vào gói để HTTPS hoạt động.
- `.github/workflows/`: build và phát hành OTA theo phê duyệt.
