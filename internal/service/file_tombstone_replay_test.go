package service

import (
	"context"
	"errors"
	"github.com/haierkeys/fast-note-sync-service/internal/domain"
	"github.com/haierkeys/fast-note-sync-service/internal/dto"
	"github.com/haierkeys/fast-note-sync-service/pkg/retirement"
	"os"
	"testing"
)

func TestUnlistedTombstoneRejectsExactBytesDespiteNewerClock(t *testing.T) {
	svc, repo, params, _ := retirementFixture(t)
	t.Setenv(retirement.EnvPolicy, "")
	svc.retirement = nil
	params.Mtime = 9999999999999
	_, _, err := svc.UploadComplete(context.Background(), 1, params)
	var rejected *RetiredBytesError
	if !errors.As(err, &rejected) || repo.writes != 0 || repo.row.Action != domain.FileActionDelete {
		t.Fatalf("replay revived tombstone: %v, writes=%d", err, repo.writes)
	}
}

func TestUnlistedTombstoneAllowsDifferentBytesWithSameWeakHash(t *testing.T) {
	svc, repo, params, _ := retirementFixture(t)
	t.Setenv(retirement.EnvPolicy, "")
	svc.retirement = nil
	if err := os.WriteFile(params.SavePath, []byte("new user work"), 0600); err != nil {
		t.Fatal(err)
	}
	_, result, err := svc.UploadComplete(context.Background(), 1, params)
	if err != nil || result.Action != "create" || repo.writes != 1 {
		t.Fatalf("new bytes rejected: %v", err)
	}
}

func TestDeleteDuringUploadRejectsReplayAtCommit(t *testing.T) {
	svc, repo, params, _ := retirementFixture(t)
	t.Setenv(retirement.EnvPolicy, "")
	svc.retirement = nil
	repo.row.Action = domain.FileActionModify
	params.ContentHash = "different-client-hash"
	mode, _, err := svc.UploadCheck(context.Background(), 1, &dto.FileUpdateCheckRequest{Vault: params.Vault, Path: params.Path, PathHash: params.PathHash, ContentHash: params.ContentHash, Mtime: params.Mtime})
	if err != nil || mode != "UpdateContent" {
		t.Fatalf("negotiation failed: %s %v", mode, err)
	}
	// Another device deletes after negotiation but before all chunks arrive.
	repo.row.Action = domain.FileActionDelete
	_, _, err = svc.UploadComplete(context.Background(), 1, params)
	var rejected *RetiredBytesError
	if !errors.As(err, &rejected) || repo.writes != 0 {
		t.Fatalf("late upload beat delete: %v", err)
	}
}
