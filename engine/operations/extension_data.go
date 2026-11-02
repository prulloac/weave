package operations

import (
	"context"
	"encoding/json"
	"errors"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

// ExtensionDataList returns opaque artifact references for a bundle.
func (e *Executor) ExtensionDataList(
	ctx context.Context,
	request contractv1.OperationRequest,
) ([]contractv1.ExtensionDataRef, error) {
	if request.NoDB {
		return nil, nil
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return nil, err
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	records, err := store.ListExtensionData(ctx, request.Path, sqlitestore.ExtensionDataFilter{DocID: request.Target})
	if err != nil {
		return nil, err
	}
	refs := make([]contractv1.ExtensionDataRef, 0, len(records))
	for _, record := range records {
		refs = append(refs, extensionDataRef(record))
	}
	return refs, nil
}

// ExtensionDataRead returns one opaque artifact and its reference.
func (e *Executor) ExtensionDataRead(
	ctx context.Context,
	request contractv1.OperationRequest,
	extension string,
	key string,
) ([]byte, contractv1.ExtensionDataRef, error) {
	if request.NoDB {
		return nil, contractv1.ExtensionDataRef{}, errors.New("extension data requires persistent storage")
	}
	dbPath, err := resolveDBPath(request.DB)
	if err != nil {
		return nil, contractv1.ExtensionDataRef{}, err
	}
	store, err := sqlitestore.OpenExisting(dbPath)
	if err != nil {
		return nil, contractv1.ExtensionDataRef{}, err
	}
	defer store.Close()
	var data []byte
	var record sqlitestore.ExtensionDataRecord
	if request.Target != "" {
		data, record, err = store.ReadExtensionData(ctx, request.Path, extension, request.Target, key)
	} else {
		data, record, err = store.ReadExtensionDataByKey(ctx, request.Path, extension, key)
	}
	if err != nil {
		return nil, contractv1.ExtensionDataRef{}, err
	}
	return data, extensionDataRef(record), nil
}

func (e *Executor) showExtensionData(
	ctx context.Context,
	request contractv1.OperationRequest,
	docID string,
) []contractv1.ExtensionDataRef {
	refs, err := e.ExtensionDataList(ctx, contractv1.OperationRequest{
		Path:   request.Path,
		DB:     request.DB,
		Target: docID,
	})
	if err != nil {
		return nil
	}
	return refs
}

func extensionDataRef(record sqlitestore.ExtensionDataRecord) contractv1.ExtensionDataRef {
	ref := contractv1.ExtensionDataRef{
		Extension: record.Extension,
		Key:       record.Key,
		MediaType: record.MediaType,
		Size:      record.Size,
		SHA256:    record.SHA256,
	}
	if len(record.Metadata) > 0 && string(record.Metadata) != "{}" {
		_ = json.Unmarshal(record.Metadata, &ref.Metadata)
	}
	return ref
}
