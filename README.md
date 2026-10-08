# Binance OTA — TrimUI Brick Pro

Ứng dụng chỉ xem dữ liệu Binance Spot, không giao dịch.

## OTA
- Release ZIP: `BinanceGia_TrimUI_StockOS_vX.YY.zip` đính kèm GitHub Release có tag `vX.YY`.
- Workflow [publish-ota.yml](.github/workflows/publish-ota.yml) tự tải ZIP, xác minh cấu trúc, tính SHA-256, tạo `manifest.json` và commit lên `main` khi Release được publish.
- URL OTA dùng trên thiết bị: `https://raw.githubusercontent.com/phongdinh91/trimui-binance-ota/main/manifest.json`.

## Bước khởi tạo ban đầu
1. Tạo GitHub Release **v0.29**, đính kèm file `BinanceGia_TrimUI_StockOS_v0.29.zip` do ChatGPT đã chuẩn bị.
2. Chờ workflow **Publish Binance OTA manifest** chạy thành công trong tab Actions, và xác nhận `manifest.json` đã được tạo.
3. Trên thẻ nhớ, giải nén gói **BinanceGia_OTA_Bootstrap_v0.28.zip** vào gốc thẻ, giữ `Apps/BinanceGia.pak/` và dữ liệu yêu thích, rồi mở app khi có Wi-Fi để thử nâng v0.29.

**Không commit manifest trước khi có Release ZIP**, vì khi đó máy sẽ nhận được một bản cập nhật chưa tồn tại.

Nếu workflow không có quyền push manifest, vào Settings → Actions → General → Workflow permissions → **Read and write permissions**, lưu thay đổi và chạy lại workflow.

## Các phiên bản sau
Tạo Release v0.30, đính kèm `BinanceGia_TrimUI_StockOS_v0.30.zip`. Workflow sẽ tự cập nhật manifest và SHA-256; không cần nhập thủ công.

Chỉ phát hành binary được xây dựng và kiểm thử từ mã nguồn tin cậy. Không bao giờ tải OTA từ nguồn không rõ.
