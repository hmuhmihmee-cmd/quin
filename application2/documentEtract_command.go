package application2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"uuid"
)

type DocumentService struct {
	docRepo DocumentRepo
	storage FileStorage
	pdfProc PDFProcessor
}

func NewDocumentService(
	docRepo DocumentRepo,
	storage FileStorage,
	pdfProc PDFProcessor,
) *DocumentService {
	return &DocumentService{
		docRepo: docRepo,
		storage: storage,
		pdfProc: pdfProc,
	}
}

// ExtractPages: Kéo file PDF từ Storage về đĩa tạm, cắt trang, và trả về dung lượng siêu nhỏ (~200KB)
func (s *DocumentService) ExtractPages(ctx context.Context, docID uuid.UUID, pageNumbers []int) ([]byte, error) {
	if len(pageNumbers) == 0 {
		return nil, errors.New("danh sách trang chọn không được để trống")
	}

	// 1. Kiểm tra tài liệu trong Database
	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy tài liệu: %w", err)
	}
	if !doc.IsReady() {
		return nil, errors.New("tài liệu chưa ở trạng thái sẵn sàng để trích xuất")
	}

	// 2. Tạo thư mục tạm trên đĩa để thao tác
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("slice-%s-*", docID))
	if err != nil {
		return nil, fmt.Errorf("lỗi tạo thư mục tạm: %w", err)
	}
	defer os.RemoveAll(tempDir) // Dọn dẹp sạch sẽ sau khi xong

	// 3. Mở Stream từ Storage và lưu tạm ra đĩa cứng (Tránh nạp 100MB vào RAM)
	storageStream, err := s.storage.OpenStream(ctx, doc.StoragePath())
	if err != nil {
		return nil, fmt.Errorf("không thể mở stream từ kho lưu trữ: %w", err)
	}
	defer storageStream.Close()

	srcTempPDFPath := filepath.Join(tempDir, "source.pdf")
	srcTempFile, err := os.Create(srcTempPDFPath)
	if err != nil {
		return nil, fmt.Errorf("không thể tạo file PDF tạm trên đĩa: %w", err)
	}

	if _, err := io.Copy(srcTempFile, storageStream); err != nil {
		srcTempFile.Close()
		return nil, fmt.Errorf("lỗi stream file PDF từ storage xuống đĩa: %w", err)
	}
	srcTempFile.Close()

	// 4. Cắt các trang chọn từ đĩa sang đĩa (pdfcpu chạy cực nhanh, chỉ vài chục ms)
	destTempPDFPath := filepath.Join(tempDir, "sliced.pdf")
	err = s.pdfProc.ExtractPagesToFile(ctx, srcTempPDFPath, destTempPDFPath, pageNumbers)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi cắt trang PDF trên đĩa: %w", err)
	}

	// 5. Đọc file kết quả đã cắt
	// File này chỉ có 1-2 trang, kích thước siêu nhỏ (~200KB - 500KB) -> Đọc vào RAM an toàn 100%!
	slicedBytes, err := os.ReadFile(destTempPDFPath)
	if err != nil {
		return nil, fmt.Errorf("lỗi đọc file PDF sau khi cắt: %w", err)
	}

	return slicedBytes, nil
}
