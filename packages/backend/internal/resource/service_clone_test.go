package resource

import (
	"context"
	"testing"
)

func TestPresignCloneImageLimit(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ok, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "cover.png", ContentType: "image/png", Size: 10 << 20, Purpose: PurposeCloneImage,
	})
	if err != nil {
		t.Fatalf("presign 10MB image: %v", err)
	}
	if repo.items[ok.ResourceID].Purpose != PurposeCloneImage {
		t.Fatalf("purpose = %q", repo.items[ok.ResourceID].Purpose)
	}
	if _, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "cover.png", ContentType: "image/png", Size: 10<<20 + 1, Purpose: PurposeCloneImage,
	}); err != ErrTooLarge {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestPresignCloneUnitTest(t *testing.T) {
	svc, _, _, _ := newTestService()
	if _, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "tests.zip", ContentType: "application/zip", Size: 50 << 20, Purpose: PurposeCloneUnitTest,
	}); err != nil {
		t.Fatalf("presign zip: %v", err)
	}
	if _, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "tests.txt", ContentType: "text/plain", Size: 1024, Purpose: PurposeCloneUnitTest,
	}); err != ErrInvalidFile {
		t.Fatalf("err = %v, want ErrInvalidFile", err)
	}
}

func TestPresignCloneDocsAllowsLargeArbitraryFile(t *testing.T) {
	svc, _, _, _ := newTestService()
	result, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "spec.custom", ContentType: "application/octet-stream", Size: 50 << 20, Purpose: PurposeCloneDocs,
	})
	if err != nil {
		t.Fatalf("presign docs: %v", err)
	}
	if result.ResourceID == "" {
		t.Fatal("resource id is empty")
	}
	if _, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "spec.custom", ContentType: "application/octet-stream", Size: 100<<20 + 1, Purpose: PurposeCloneDocs,
	}); err != ErrTooLarge {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestAdoptCloneDocsUsesCloneProjectLimit(t *testing.T) {
	svc, repo, s3, _ := newTestService()
	const size = 50 << 20
	presign, err := svc.Presign(context.Background(), 1, 2, PresignInput{
		Filename: "spec.pdf", ContentType: "application/pdf", Size: size, Purpose: PurposeCloneDocs,
	})
	if err != nil {
		t.Fatal(err)
	}
	s3.size = size
	s3.contentType = "application/pdf"
	if _, err := svc.Adopt(context.Background(), 1, 2, "p1", presign.ResourceID); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if repo.items[presign.ResourceID].Status != "ready" {
		t.Fatalf("status = %q", repo.items[presign.ResourceID].Status)
	}
	if repo.items[presign.ResourceID].Purpose != PurposeCloneDocs {
		t.Fatalf("purpose = %q", repo.items[presign.ResourceID].Purpose)
	}
}
