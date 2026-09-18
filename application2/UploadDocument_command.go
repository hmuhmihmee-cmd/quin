package application2

import (
	"context"
	"fmt"
	"io"
	"meet-attendance-clean/domain2/document"
	"os"
	"path/filepath"
	"strings"
	"uuid"
)

// ==========================================================
// 1. PORTS (INTERFACES STREAMING & FILE-BASED)
// ==========================================================

// FileStorage: Hỗ trợ Stream để không bao giờ tốn RAM nạp cả file
type FileStorage interface {
	SaveStream(ctx context.Context, storagePath string, reader io.Reader) error
	OpenStream(ctx context.Context, storagePath string) (io.ReadCloser, error)
}

// DocumentConverter: Nhận Stream Word, trả về Stream PDF
type DocumentConverter interface {
	ConvertWordToPDF(ctx context.Context, docxStream io.Reader) (io.ReadCloser, error)
}

// PDFProcessor: Thao tác trực tiếp trên đĩa cứng (Disk-based)
type PDFProcessor interface {
	// Đọc file PDF từ đĩa, render các trang thành ảnh lưu vào outputDir, trả về danh sách đường dẫn ảnh
	RenderPagesToDisk(ctx context.Context, pdfFilePath string, outputDir string) (pageCount int, imagePaths []string, err error)
	// Cắt trang trực tiếp từ file PDF nguồn sang file PDF đích trên đĩa
	ExtractPagesToFile(ctx context.Context, srcPDFPath, destPDFPath string, pages []int) error
}

type DocumentRepo interface {
	Save(ctx context.Context, doc *document.SourceDocument) error
	GetByID(ctx context.Context, id uuid.UUID) (*document.SourceDocument, error)
}

type Logger interface {
	Error(ctx context.Context, msg string, err error, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
}

// ==========================================================
// 2. COMMAND (NHẬN STREAM, KHÔNG NHẬN []BYTE)
// ==========================================================

type UploadDocumentCommand struct {
	FileName   string
	FileStream io.Reader // Stream trực tiếp từ r.Body / multipart.File
}

// ==========================================================
// 3. USECASE IMPLEMENTATION
// ==========================================================

type UploadDocumentUsecase struct {
	docRepo   DocumentRepo
	storage   FileStorage
	converter DocumentConverter
	pdfProc   PDFProcessor
	logger    Logger
}

func NewUploadDocumentUsecase(
	docRepo DocumentRepo,
	storage FileStorage,
	converter DocumentConverter,
	pdfProc PDFProcessor,
	logger Logger,
) *UploadDocumentUsecase {
	return &UploadDocumentUsecase{
		docRepo:   docRepo,
		storage:   storage,
		converter: converter,
		pdfProc:   pdfProc,
		logger:    logger,
	}
}

func (u *UploadDocumentUsecase) Upload(ctx context.Context, cmd UploadDocumentCommand) (uuid.UUID, error) {
	// BƯỚC 1: Khởi tạo Entity Document trong DB
	doc := document.NewSourceDocument(cmd.FileName)
	if err := u.docRepo.Save(ctx, doc); err != nil {
		return uuid.Nil(), fmt.Errorf("không thể khởi tạo tài liệu trong DB: %w", err)
	}

	// Helper xử lý lỗi & đổi trạng thái Failed
	markDocAsFailed := func(causeErr error, phase string) error {
		doc.MarkAsFailed(causeErr)
		if dbErr := u.docRepo.Save(ctx, doc); dbErr != nil {
			u.logger.Error(ctx, "CRITICAL: Không thể lưu trạng thái FAILED của document vào DB", dbErr,
				"document_id", doc.ID(),
				"original_cause", causeErr.Error(),
			)
		}
		return fmt.Errorf("tiến trình thất bại tại bước [%s]: %w", phase, causeErr)
	}

	// BƯỚC 2: Tạo thư mục làm việc tạm thời trên đĩa cứng (/tmp)
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("doc-%s-*", doc.ID()))
	if err != nil {
		return uuid.Nil(), markDocAsFailed(err, "Tạo thư mục tạm trên đĩa")
	}
	// Dọn sạch toàn bộ file tạm khi thoát khỏi hàm (cho dù thành công hay lỗi)
	defer os.RemoveAll(tempDir)

	var localPDFPath string

	// BƯỚC 3: Xử lý Upload & Chuyển đổi Word -> PDF (Hoàn toàn bằng Stream)
	if strings.HasSuffix(strings.ToLower(cmd.FileName), ".docx") {
		// Gọi bên thứ 3 chuyển đổi Word sang PDF
		pdfReadCloser, err := u.converter.ConvertWordToPDF(ctx, cmd.FileStream)
		if err != nil {
			return uuid.Nil(), markDocAsFailed(err, "Convert Word sang PDF")
		}
		defer pdfReadCloser.Close()

		// Ghi stream PDF kết quả ra file tạm trên đĩa
		localPDFPath = filepath.Join(tempDir, "converted.pdf")
		outFile, err := os.Create(localPDFPath)
		if err != nil {
			return uuid.Nil(), markDocAsFailed(err, "Tạo file PDF tạm sau convert")
		}
		if _, err := io.Copy(outFile, pdfReadCloser); err != nil {
			outFile.Close()
			return uuid.Nil(), markDocAsFailed(err, "Ghi file PDF sau convert xuống đĩa")
		}
		outFile.Close()
	} else {
		// Nếu là PDF sẵn: Stream thẳng từ HTTP ghi xuống file tạm trên đĩa
		localPDFPath = filepath.Join(tempDir, "original.pdf")
		outFile, err := os.Create(localPDFPath)
		if err != nil {
			return uuid.Nil(), markDocAsFailed(err, "Tạo file PDF tạm từ upload")
		}
		if _, err := io.Copy(outFile, cmd.FileStream); err != nil {
			outFile.Close()
			return uuid.Nil(), markDocAsFailed(err, "Stream file PDF từ upload xuống đĩa")
		}
		outFile.Close()
	}

	// BƯỚC 4: Stream file PDF từ đĩa lên Storage lưu trữ lâu dài (S3/MinIO/Persistent Disk)
	pdfFileToStore, err := os.Open(localPDFPath)
	if err != nil {
		return uuid.Nil(), markDocAsFailed(err, "Mở file PDF tạm để lưu vào Storage")
	}
	defer pdfFileToStore.Close()

	storagePDFPath := fmt.Sprintf("documents/%s/document.pdf", doc.ID())
	if err := u.storage.SaveStream(ctx, storagePDFPath, pdfFileToStore); err != nil {
		return uuid.Nil(), markDocAsFailed(err, "Lưu file PDF vào Storage")
	}

	// BƯỚC 5: Render các trang thành ảnh ngay trên đĩa cứng
	previewOutputDir := filepath.Join(tempDir, "previews")
	if err := os.MkdirAll(previewOutputDir, 0755); err != nil {
		return uuid.Nil(), markDocAsFailed(err, "Tạo thư mục tạm chứa ảnh preview")
	}

	pageCount, imagePaths, err := u.pdfProc.RenderPagesToDisk(ctx, localPDFPath, previewOutputDir)
	if err != nil {
		return uuid.Nil(), markDocAsFailed(err, "Render PDF thành hình ảnh trên đĩa")
	}

	// BƯỚC 6: Stream từng ảnh từ đĩa lên Storage và thu thập URL
	var previewURLs []string
	for i, localImgPath := range imagePaths {
		imgFile, err := os.Open(localImgPath)
		if err != nil {
			return uuid.Nil(), markDocAsFailed(err, fmt.Sprintf("Mở ảnh preview trang %d", i+1))
		}

		storageImgPath := fmt.Sprintf("documents/%s/previews/page_%d.webp", doc.ID(), i+1)
		err = u.storage.SaveStream(ctx, storageImgPath, imgFile)
		imgFile.Close() // Đóng file ngay sau khi stream xong

		if err != nil {
			return uuid.Nil(), markDocAsFailed(err, fmt.Sprintf("Lưu ảnh preview trang %d vào Storage", i+1))
		}
		previewURLs = append(previewURLs, storageImgPath)
	}

	// BƯỚC 7: Cập nhật Document sang trạng thái READY
	doc.MarkAsReady(storagePDFPath, pageCount, previewURLs)
	if err := u.docRepo.Save(ctx, doc); err != nil {
		u.logger.Error(ctx, "Lỗi out-of-sync: Files đã lưu Storage nhưng không cập nhật được DB thành READY", err, "document_id", doc.ID())
		return uuid.Nil(), fmt.Errorf("không thể hoàn tất quy trình lưu tài liệu: %w", err)
	}

	u.logger.Info(ctx, "Upload và xử lý tài liệu thành công (Zero RAM spike)", "document_id", doc.ID(), "pages", pageCount)
	return doc.ID(), nil
}
