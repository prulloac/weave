package operations

import (
	"context"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func (e *Executor) multiWrite(
	ctx context.Context,
	request contractv1.OperationRequest,
	sources []resolvedSource,
) any {
	reports := make([]contractv1.SourceReport, 0, len(sources))
	var aggregate any
	for _, source := range sources {
		value, failed := unwrap(e.Execute(ctx, singleSourceRequest(request, source)))
		if failed != nil {
			return *failed
		}
		switch report := value.(type) {
		case contractv1.IngestReport:
			report.Sources = nil
			reports = append(reports, contractv1.SourceReport{
				Source:    source.Alias,
				Documents: report.Documents,
				Warnings:  report.Warnings,
				Errors:    report.Errors,
			})
			if aggregate == nil {
				report.Bundle = request.Path
				aggregate = report
			}
		case contractv1.SyncReport:
			report.Sources = nil
			reports = append(reports, contractv1.SourceReport{
				Source:    source.Alias,
				Documents: report.Documents,
				Added:     report.Added,
				Updated:   report.Updated,
				Removed:   report.Removed,
				Skipped:   report.Skipped,
				Warnings:  report.Warnings,
				Errors:    report.Errors,
			})
			if aggregate == nil {
				report.Bundle = request.Path
				aggregate = report
			}
		default:
			return failure(contractv1.ErrInternal, "unexpected write payload", nil)
		}
	}

	switch report := aggregate.(type) {
	case contractv1.IngestReport:
		report.Documents = 0
		report.Edges = 0
		report.Synthetic = 0
		report.Warnings = 0
		report.Errors = 0
		for _, part := range reports {
			report.Documents += part.Documents
			report.Warnings += part.Warnings
			report.Errors += part.Errors
		}
		report.Sources = reports
		return success(report)
	case contractv1.SyncReport:
		report.Documents = 0
		report.Added = 0
		report.Updated = 0
		report.Removed = 0
		report.Skipped = 0
		report.Warnings = 0
		report.Errors = 0
		for _, part := range reports {
			report.Documents += part.Documents
			report.Added += part.Added
			report.Updated += part.Updated
			report.Removed += part.Removed
			report.Skipped += part.Skipped
			report.Warnings += part.Warnings
			report.Errors += part.Errors
		}
		report.Sources = reports
		return success(report)
	default:
		return failure(contractv1.ErrInternal, "unexpected write payload", nil)
	}
}
