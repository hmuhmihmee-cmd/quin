# Lớp học 1–1

Ứng dụng Go một file thực thi dành cho giáo viên dạy Google Meet:

- đồng bộ các conference record, người tham gia và phiên vào/ra trong 30 ngày gần nhất;
- tự tạo một học sinh cho mỗi Google Space, mặc định tên `Học sinh mới N` và chu kỳ ngày 1;
- lưu ID Google thuần, không lưu các tiền tố `conferenceRecords/` và `spaces/`;
- thống kê mỗi học sinh theo Google Space cố định và chu kỳ tháng riêng;
- tìm học sinh theo tên, chọn tháng hoặc khoảng ngày;
- xem từng buổi học, thời lượng buổi và thời lượng thực tế của từng người tham gia;
- tạo hai bản Markdown từ video YouTube bằng Gemini và prompt tùy chỉnh;
- chỉnh sửa Markdown bằng trình soạn thảo WYSIWYG trước khi tải `.md` hoặc xuất hai PDF.

Không còn nghiệp vụ học phí hoặc trạng thái tính tiền.

## Mô hình dữ liệu

Domain chỉ có ba entity phẳng:

```text
Student --google_space_name--> Meeting --meeting_id--> Participant
```

SQLite có đúng ba bảng tương ứng: `students`, `meetings`, `participants`. SQL được sinh bằng sqlc. Thời lượng participant là hợp của các session sau khi cắt theo thời gian conference, nên không bị cộng đôi khi một người dùng nhiều thiết bị.

## Chạy và build

```bash
sqlc generate
go test ./...
go build -buildvcs=false -o meet-attendance .
./meet-attendance
```

Mở `http://localhost:9000`.

Build Windows:

```bash
GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags="-s -w" -o meet-attendance.exe .
```

Gemini API key, Google OAuth config, HTML, CSS, cấu hình mặc định và font PDF đều được nhúng bằng `go:embed`. Chị bạn chỉ cần chạy `meet-attendance.exe`. Ứng dụng tự tạo `attendance.db` và `token.json` cạnh file chạy.

Lưu ý: API key có thể bị trích xuất từ binary. Chỉ phát hành riêng; không đưa binary hoặc source chứa key lên nơi công khai.

## Hướng phụ thuộc

```text
domain <- application <- infrastructure <- main
```

Struct phản hồi Google Meet chỉ nằm trong `infrastructure/googlemeet`. Domain không phụ thuộc kiểu dữ liệu của Google.
