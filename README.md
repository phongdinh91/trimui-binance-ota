# BINANCE — TrimUI Brick Pro (Stock OS)

## Bản v0.32 — BINANCE, KINH DOANH, HI-TECH

- Có **3 tab chính**: BINANCE → KINH DOANH → HI-TECH. Chuyển bằng **L1/R1**.
- **BINANCE (Yêu thích):** D-pad chọn cặp, **A: MỞ**, **START: TÌM KIẾM**, **SELECT: BỎ SAO**.
- **BINANCE (Tìm kiếm):** **A: NHẬP**, **Y: ĐỔI VÙNG**, **START: ẨN**, **SELECT: YÊU THÍCH**. A chỉ gõ bằng bàn phím ảo, **không mở coin**. SELECT trên gợi ý bật/tắt dấu sao. START đóng ô tìm kiếm, **về đúng màn lưới BINANCE**, không tự quay lại biểu đồ cặp vừa xem.
- Chữ phiên bản bên cạnh BINANCE đã chỉnh **trái 1 px, xuống 2 px**.
- **KINH DOANH:** lưới **12 ô (2 cột, cuộn theo dòng)** liên kết danh mục thực tế từ 24h.com.vn: Giá vàng, Tỷ giá, Chứng khoán, Kinh tế thế giới, Bất động sản, Doanh nhân, Nhà đẹp, Khởi nghiệp, Ngân hàng, Quản lý tiền, Doanh nghiệp, Thị trường. D-pad chọn mục, **A** mở nội dung, **B** quay lại; mục Giá vàng giữ bảng giá hiện có.
- **HI-TECH:** lưới **12 ô** theo 24h.com.vn: Hi-tech, Điện thoại, Đánh giá, Laptop, Tin công nghệ, Điện tử gia dụng, Máy tính bảng, Bảng giá điện thoại, iPhone 17, Galaxy A, iPhone, Samsung Galaxy. D-pad, A, B giống tab KINH DOANH.
- Trong danh mục báo, app tải danh sách tiêu đề tin; A mở trang **trích yếu** và B quay lại. Trích yếu phụ thuộc vào cấu trúc HTML, không thay thế trình duyệt đọc bài đầy đủ. X tải lại. Nếu mất kết nối/trang đổi cấu trúc, hiện thông báo thay vì tin giả.
- OTA vẫn có tiến trình tải ZIP, kiểm tra SHA-256, giải nén và cài đặt; dữ liệu yêu thích/cài đặt vẫn được giữ lại.

**Kiểm thử:** Các bài test Go và build Linux ARM64 qua GitHub Actions; vẫn cần kiểm tra bố cục và kết nối 24h trên TrimUI Brick Pro thực tế.


## v0.31 — BINANCE gộp tìm kiếm và yêu thích; OTA có tiến trình

- **Hai tab chính:** **BINANCE** và **KINH DOANH**. Điều hướng bằng **L1/R1**.
- Trong **BINANCE**, mặc định là thẻ hai cột của các cặp yêu thích; dùng D-pad và **A** để xem biểu đồ, **SELECT** bỏ sao.
- **START** bật ô tìm kiếm và bàn phím ảo ngay trong BINANCE. Gõ cặp cần tìm, dùng **Y** để chuyển vùng bàn phím ↔ gợi ý, di chuyển đến cặp và bấm **A**: bàn phím tự ẩn, biểu đồ cặp đó hiển thị ngay trong BINANCE. **SELECT** trên gợi ý bật/tắt yêu thích. **B** xóa ký tự, **X** xóa hết.
- **START lần nữa** mở ô tìm kiếm khi đang xem biểu đồ hoặc yêu thích; nếu đang tìm kiếm thì START ẩn bàn phím và trở lại biểu đồ vừa mở (nếu có), hoặc danh sách yêu thích.
- Tab KINH DOANH vẫn lấy bảng giá vàng từ 24h.com.vn; **lên/xuống** chọn dòng, **X** cập nhật giá.
- **OTA:** thông báo cập nhật có 4 trạng thái tải ZIP, xác thực SHA-256, giải nén, cài file; tải ZIP hiển thị số KB đã nhận và % thật khi máy chủ cung cấp dung lượng; giải nén/cài đặt hiển thị số file đã xử lý trên tổng số file. Không hiển thị phần trăm giả nếu chưa biết tổng kích thước. Có thông báo hoàn tất hoặc lỗi và nút khởi động lại.
- Các file yêu thích, cài đặt và cache không nằm trong ZIP phát hành và được giữ nguyên khi OTA.

**Chưa kiểm chứng trên thiết bị thật:** kiểm thử tự động và bản build ARM64 được thực hiện bằng GitHub Actions, nhưng bố cục nút và tốc độ OTA vẫn cần thử trên Brick Pro.


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

## Mới trong v0.30 — tab KINH DOANH

Tab **KINH DOANH** xem bảng giá vàng từ [24h.com.vn](https://www.24h.com.vn/gia-vang-hom-nay-c425.html) (SJC, DOJI, BTMH, BTMC và PNJ): cột **MUA**, **BÁN** theo đơn vị **triệu đồng/lượng**. Giá trang 24h công bố theo nghìn đồng/lượng, ứng dụng chia 1.000 khi hiển thị triệu đồng/lượng. Tab có thời gian cập nhật nguồn, tự yêu cầu tải lại mỗi 5 phút khi đang xem và giữ giá trước đó kèm cảnh báo khi tải lỗi.

Điều khiển trên Brick Pro: **R1** lần lượt **TÌM KIẾM → YÊU THÍCH → KINH DOANH**, **L1** theo chiều ngược lại (có vòng lặp); trong KINH DOANH **lên/xuống** cuộn danh sách, **X** tải lại, không có thao tác giao dịch. Hai tab Binance trước đó và dữ liệu yêu thích tiếp tục hoạt động.

**Giới hạn:** dữ liệu vàng đến từ trang web công khai của 24h, không phải API có cam kết. Nếu trang đổi HTML/chặn HTTP hoặc mất Wi-Fi, ứng dụng hiển thị lỗi thay vì tự đoán số. Đã kiểm thử bộ phân tích với HTML mẫu và biên dịch ARM64 trên GitHub Actions; cần xác nhận chức năng hiển thị trên Brick Pro thật. Không sử dụng số liệu này để đặt lệnh tự động.
