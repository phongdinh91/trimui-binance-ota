# BINANCE — TrimUI Brick Pro (Stock OS)

## v0.38 — LED Studio tích hợp vào BINANCE

- **Cài đặt → Điều khiển LED:** 5 hàng gồm Chọn hiệu ứng, Độ sáng, Tốc độ, Màu chính và Màu phụ. D-pad **trái giảm / phải tăng** theo tốc độ thực; tên là **TỐC ĐỘ**, không phải tốc độ nháy.
- Giữ nguyên mã cũ của các chế độ đã lưu. Có **9 nhóm**: màu tĩnh (5 màu cũ), thở chậm, báo mức pin, chuyển sắc đôi, vòng màu, ánh sáng dịu, cầu vồng, chạy đuổi và phản hồi phím; chế độ nhấp nháy cũ vẫn khả dụng.
- Driver TG4040 có thể giới hạn `frame_hex` ở dải đỉnh sau. Các hiệu ứng LED Studio mới **không ghi `frame_hex` liên tục**; thay vào đó, phát màu tách biệt trên mọi node `effect_rgb_hex_<zone>` mà firmware công bố, kết hợp chế độ native static 4. Vùng nào không có node sẽ không được giả lập là đã điều khiển. Firmware chỉ công bố một vùng sẽ báo lỗi, không báo thành công giả.
- Phản hồi nút bấm được chuyển từ vòng xử lý gamepad của BINANCE sang tiến trình LED. Chế độ báo pin đọc `/sys/class/power_supply/*/capacity`; không có dữ liệu pin sẽ báo lỗi. Hai chế độ chuyển sắc đôi/ánh sáng dịu giữ màu tĩnh để giảm truy cập driver; các chế độ động dùng tốc độ giới hạn 160–700 ms.
- Cài đặt được lưu trong `settings.json`; OTA giữ file này và chỉ kiểm tra khi bấm **Cài đặt → Cập nhật OTA**.
- **Thận trọng:** Firmware có thể gộp nhiều bóng LED trong một node (như `lr`) nên chuyển sắc ở mức vùng, không thể giả định điều khiển riêng từng điểm ảnh trong vòng analog. Chưa thử trên máy TG4040 thật. Dừng hiệu ứng nếu xảy ra treo hoặc quá nhiệt, trở về HỆ THỐNG và khởi động lại. Gửi kết quả từ `scripts/probe-led-brickpro.sh` để hiệu chỉnh ánh xạ.

## v0.37 — Sửa vùng LED Brick Pro và hướng D-pad

- Đổi nhãn **TỐC ĐỘ NHÁY** thành **TỐC ĐỘ**.
- Chỉnh hướng điều khiển tốc độ: **D-pad PHẢI = NHANH HƠN**, **D-pad TRÁI = CHẬM HƠN**. Giữ nguyên thứ tự mức tốc độ đã lưu trong `settings.json`; không đảo ngược tốc độ cũ sau khi cập nhật.
- Sửa hiệu ứng **CẦU VỒNG** và **CHẠY ĐUỔI**: v0.36 chỉ xuất **8 màu** cho một cụm LED. v0.37 tạo khung cho **23 vị trí trên driver Brick Pro (TG4040)** hoặc **14 vị trí trên Brick thường**, và lần lượt chạy qua toàn bộ vị trí trong hiệu ứng đuổi.
- Trước hoạt ảnh, bật lại độ sáng cho các vùng phần cứng có node tương ứng: `max_scale`, `max_scale_f1f2`, `max_scale_lr`, `max_scale_rear`; đồng thời xóa màu nền hiệu ứng cũ ở các vùng `m`, `f1`, `f2`, `lr`, `rear`, `l`, `r` có hỗ trợ để tránh vùng bị tắt hoặc nhấp nháy khi chuyển khung.
- Giữ cơ chế an toàn `effect_enable: 1 → 0` giữa các lần ghi `frame_hex`, phát hoạt ảnh trong tiến trình riêng, có nút xác nhận A trước khi thử và dừng tiến trình khi thoát.

**Chưa thể xác nhận đủ mọi vùng LED trên phần cứng Brick Pro chỉ bằng GitHub Actions.** Kết quả cần được thử thực tế trên Stock OS TG4040; sơ đồ LED và số vị trí có thể thay đổi theo firmware. Nếu chỉ một vùng sáng hoặc thiết bị không phản hồi, ngừng thử hiệu ứng, chuyển về HỆ THỐNG và khởi động lại máy. Nếu lỗi còn, cần thông tin firmware và `/sys/class/led_anim/help` để đối chiếu bố trí LED thực tế. 


## v0.36 — Trang con LED, hiệu ứng cầu vồng và chạy đuổi

**CÀI ĐẶT → ĐIỀU KHIỂN LED → A** mở trang riêng gồm ba lựa chọn:

- **CHỌN HIỆU ỨNG:** HỆ THỐNG, TẮT, màu tĩnh, NHỊP THỞ, NHẤP NHÁY, **CẦU VỒNG**, **CHẠY ĐUỔI**. D-pad trái/phải đổi hiệu ứng.
- **ĐỘ SÁNG:** 10–60, tăng giảm 10 mỗi lần.
- **TỐC ĐỘ NHÁY:** 5 cấp, từ RẤT NHANH đến RẤT CHẬM.
- **Lên/xuống** chọn mục; **B** trở về CÀI ĐẶT. Các lựa chọn áp dụng thành công được lưu vào `settings.json`.

**CẦU VỒNG / CHẠY ĐUỔI:** thực hiện hoạt ảnh trên **8 LED thanh trên** bằng chế độ `frame_hex`, chỉ khi firmware có đủ các file điều khiển và cho phép ghi. Để tránh kích hoạt vô ý, sau khi dùng trái/phải chọn một trong hai hiệu ứng thử nghiệm, bạn **phải bấm A xác nhận**. Ứng dụng khởi chạy một tiến trình LED tách biệt với giao diện, kiểm tra tín hiệu READY trong 3 giây; khi rời app hoặc chuyển hiệu ứng sẽ yêu cầu dừng tiến trình. Khi mở app lại, hiệu ứng thử nghiệm **không tự chạy** cho đến khi người dùng xác nhận lại.

**Cảnh báo phần cứng:** driver LED trên một số firmware TrimUI Brick có lỗi *treo khi ghi `frame_hex` liên tục*. Bộ hoạt ảnh tuân theo cách khắc phục đã được cộng đồng kiểm thử (luôn chuyển `effect_enable` về 1 rồi 0 giữa mỗi lần ghi), nhưng **chưa được xác minh trên Brick Pro Stock OS của bạn**. Nếu firmware không tương thích, đừng thử lại liên tục; chuyển về HỆ THỐNG và khởi động lại máy. Hiệu ứng trên tab này là thử nghiệm, không đảm bảo hoạt động với mọi bản firmware. Bản build/kiểm thử tự động không kiểm tra được driver thật.

Bố cục khác được giữ từ v0.35: **24h.com.vn trong thanh trên cùng** của KINH DOANH và HI-TECH; chữ trong CÀI ĐẶT cỡ lớn; OTA chỉ kiểm tra khi bấm trong CÀI ĐẶT.


## v0.35 — 24h trong thanh trên, LED mở rộng và điều chỉnh sáng/nháy

- Trên **KINH DOANH** và **HI-TECH**, nguồn `24h.com.vn` được hiển thị **bên trong thanh trên cùng** bên dưới tên tab; thanh điều hướng bên dưới chỉ còn tên bốn tab.
- **CÀI ĐẶT**: tên các mục tăng lên cỡ chữ 3x, in đậm, có ô chọn rõ ràng. Gồm 6 hàng: Cập nhật OTA thủ công, Chủ đề sáng/tối, Hiệu ứng LED, Độ sáng LED, Tốc độ nháy, Giới thiệu.
- **Hiệu ứng LED** thêm: Nhấp nháy, Cầu vồng, Chạy đuổi, Đỏ tĩnh, Lục tĩnh; vẫn giữ Hệ thống, Tắt, Vàng, Xanh, Tím và Nhịp thở. Chọn bằng **A** hoặc **D-pad trái/phải** ở hàng Hiệu ứng LED.
- **Độ sáng** dùng thang giá trị sysfs `max_scale` từ **10 đến 60**, bước 10. **Tốc độ** có 5 mức (rất nhanh đến rất chậm), tương ứng driver native `effect_duration_*` từ **200 đến 1600 ms**. Chọn hàng rồi bấm trái/phải hoặc A để tăng.
- **Không dùng vòng lặp ghi vào `frame_hex`** do firmware Brick có nguy cơ treo tiến trình khi ghi liên tục. Các hiệu ứng động chỉ được áp dụng nếu `effect_names` công bố tên hiệu ứng và `effect_duration_m` tồn tại; nếu không, app báo lỗi và không lưu lựa chọn không hỗ trợ.
- Các mức sáng, tốc độ và chế độ LED được lưu vào `settings.json` và có giá trị mặc định an toàn khi nâng cấp từ bản cũ. **Cài đặt → Cập nhật OTA** vẫn là thao tác thủ công, không kiểm tra tự động khi mở app.

**Lưu ý thiết bị:** Tính năng màu, độ sáng và tốc độ LED cần được xác minh trên firmware của Brick Pro thật. Các tùy chọn chỉ sử dụng giao diện native `/sys/class/led_anim/` có trên firmware tương thích; hiệu ứng hiển thị có thể khác nhau theo phiên bản OS.


## v0.34 — CÀI ĐẶT và OTA thủ công

**Bốn tab chính:** BINANCE, KINH DOANH, HI-TECH và **CÀI ĐẶT**, chuyển qua lại bằng L1/R1.

- **Phiên bản** xuất hiện ở **thanh ngang trên cùng**, ngay dưới đồng hồ và ngày, dùng một bố cục chung trên tất cả các tab; không hiển thị phiên bản ở chân màn hình.
- **Chú thích nút** căn từ **mép trái**, các nhóm cách nhau vừa đủ để đọc. Không còn chia đều thành các cột cách xa nhau.
- **Nguồn 24h:** dưới tiêu đề hai tab KINH DOANH và HI-TECH có chữ **24h.com.vn**.
- **CÀI ĐẶT → CẬP NHẬT OTA:** bấm A để kiểm tra phiên bản từ manifest GitHub. App không kiểm tra OTA và không hiện thông báo cập nhật lúc khởi động, kể cả nếu còn file `ota.json` cũ từng bật `check_on_start`. Nếu có bản mới, người dùng chọn CẬP NHẬT để tải và cài đặt; nếu chưa có hoặc mất mạng, màn hình báo trạng thái rõ ràng.
- **CÀI ĐẶT → CHỦ ĐỀ:** bấm A hoặc trái/phải để đổi công tắc **TỐI ⇄ SÁNG**; thay bảng màu chung cho các màn chính, trình đọc tin, thanh tab và biểu đồ. Thiết lập lưu trong `settings.json` và giữ qua OTA.
- **CÀI ĐẶT → HIỆU ỨNG LED:** bấm A hoặc trái/phải để chọn **HỆ THỐNG**, **TẮT**, **VÀNG TĨNH**, **XANH TĨNH**, **TÍM TĨNH**, **NHỊP THỞ**. Chỉ ghi qua sysfs `/sys/class/led_anim/` nếu firmware có các node phù hợp và có quyền truy cập. Chế độ nhịp thở chỉ khả dụng khi driver công bố tên hiệu ứng và ID tương ứng; nếu không có, app báo lỗi. Không sử dụng driver `frame_hex` có nguy cơ treo khi ghi lặp. Trở lại **HỆ THỐNG** có thể cần **khởi động lại** để lấy lại hiệu ứng do Stock OS quản lý. Các tùy chọn được lưu và khởi áp dụng lại nếu đã chọn.
- **CÀI ĐẶT → GIỚI THIỆU:** thông tin ứng dụng, phiên bản, thiết bị, kho mã GitHub `phongdinh91/trimui-binance-ota`, nguồn Binance Spot và tin từ 24h.com.vn.

**Lưu ý:** Tính năng LED phụ thuộc vào firmware thực tế của Brick Pro, chưa được kiểm thử trên máy thật. Nếu không tương thích, để chế độ HỆ THỐNG. Công tắc chủ đề được kiểm thử trên framebuffer ảo và có thể cần tinh chỉnh một vài biểu tượng ảnh có màu cố định. OTA vẫn yêu cầu người dùng chấp thuận cài đặt.


## v0.33 — đồng bộ giao diện, đọc bài viết có ảnh và video

- **Phiên bản:** chữ `v0.33` xuất hiện ở góc dưới bên phải, màu và cỡ nhất quán ở các tab; không còn cạnh tiêu đề BINANCE.
- **Thanh trên:** BINANCE, KINH DOANH và HI-TECH dùng chung font, cỡ chữ, màu chữ, ngày và giờ; đồng hồ tự cập nhật khi sang phút mới. Chỉ BINANCE hiển thị thêm trạng thái TRỰC TUYẾN/MẤT MẠNG.
- **Thanh chú thích:** các nhóm nút A, B, X, Y, START, SELECT... được chia khoảng đều nhau trên toàn chiều ngang. Kể cả màn biểu đồ và trình đọc tin.
- **Ô danh mục:** cỡ chữ lớn, in đậm, tên dài được ngắt tối đa hai dòng thay vì thu quá nhỏ.
- **Bài viết 24h:** cố gắng tải các đoạn nội dung từ phần thân bài (không chỉ meta description), có cuộn bằng lên/xuống, tiêu đề 3x đậm và ảnh JPG/PNG/GIF hiển thị trực tiếp từ nguồn 24h. Nếu nguồn trả về HTML giới hạn, trang đổi cấu trúc hoặc ảnh dùng định dạng khác, giao diện sẽ hiện thông báo thay vì tự tạo nội dung.
- **Video:** nhận diện video nhúng/MP4/HLS và hiển thị ảnh xem trước nếu nguồn cung cấp. Khi trỏ tới đoạn video và bấm A, có thể mở luồng MP4/HLS bằng MPV **nếu máy có trình phát MPV**. TrimUI Stock OS không được bảo đảm tích hợp MPV và phần lớn video nhúng cần trình duyệt nên có thể **chỉ xem được ảnh xem trước**. Ứng dụng nêu rõ lý do nếu không thể phát, không báo phát thành công giả.
- **Kiểm thử:** bộ parser tin, ảnh/video, bộ chú thích nút và build Linux ARM64 qua GitHub Actions. Vẫn cần thử giao diện và nội dung 24h trên Brick Pro thật.


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
