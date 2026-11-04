package operations

import (
	"context"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/codecs"
)

func (e *Executor) codecList(ctx context.Context, request contractv1.OperationRequest) any {
	path := request.Path
	if path == "" {
		path = "."
	}
	infos, err := codecs.New(path, codecs.DefaultTimeout).List(ctx)
	if err != nil {
		return failure(contractv1.ErrCodecFailed, err.Error(), map[string]any{"path": path})
	}
	result := contractv1.CodecList{Codecs: make([]contractv1.CodecInfo, 0, len(infos))}
	for _, info := range infos {
		result.Codecs = append(result.Codecs, contractv1.CodecInfo{
			Name:          info.Name,
			Source:        info.Source,
			Path:          info.Path,
			CanCompress:   info.CanCompress,
			CanDecompress: info.CanDecompress,
		})
	}
	return success(result)
}
