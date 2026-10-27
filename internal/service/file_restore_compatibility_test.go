package service

import (
	"context"
	"github.com/haierkeys/fast-note-sync-service/internal/domain"
	"github.com/haierkeys/fast-note-sync-service/internal/dto"
	"testing"
)

// Without an explicitly configured retirement policy, the existing protocol
// cannot distinguish a local trash/Git restore from stale offline content.
// Byte equality must therefore not become an implicit restore prohibition.
func TestUnconfiguredPolicyAllowsLocalRestoreOfIdenticalBytes(t *testing.T) {
	svc, repo, params, _ := retirementFixture(t)
	svc.retirement = nil
	params.Mtime = 9999999999999
	mode, _, err := svc.UploadCheck(context.Background(), 1, &dto.FileUpdateCheckRequest{
		Vault: params.Vault, Path: params.Path, PathHash: params.PathHash,
		ContentHash: params.ContentHash, Mtime: params.Mtime,
	})
	if err != nil || mode != "Create" {
		t.Fatalf("restore negotiation failed: %s %v", mode, err)
	}
	_, result, err := svc.UploadComplete(context.Background(), 1, params)
	if err != nil || result == nil || result.Action != string(domain.FileActionCreate) || repo.writes != 1 {
		t.Fatalf("identical-byte local restore was rejected: result=%+v err=%v writes=%d", result, err, repo.writes)
	}
	if repo.row.SavePath != params.SavePath {
		t.Fatal("restored upload bytes were discarded")
	}
}

func TestUnconfiguredPolicyDoesNotInferRestoreIntentFromWeakHashOrClock(t *testing.T) {
	for _, mtime := range []int64{1, 9999999999999} {
		t.Run(map[bool]string{true: "older-clock", false: "newer-clock"}[mtime == 1], func(t *testing.T) {
			svc, repo, params, _ := retirementFixture(t)
			svc.retirement = nil
			params.Mtime = mtime
			_, result, err := svc.UploadComplete(context.Background(), 1, params)
			if err != nil || result == nil || result.Action != "create" || repo.writes != 1 {
				t.Fatalf("normal restore blocked based on equal bytes/clock: %+v %v", result, err)
			}
		})
	}
}
