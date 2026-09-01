package model

// CommonPageQuery 对应 Java 的 CommonPageQuery
type CommonPageQuery struct {
	PageNum  *int `json:"pageNum,omitempty"`
	PageSize *int `json:"pageSize,omitempty"`
}

// PaginationEnabled 判断是否启用分页
func (q *CommonPageQuery) PaginationEnabled() bool {
	return q != nil && (q.PageNum != nil || q.PageSize != nil)
}

// PaginatedResult 对应 Java 的 PaginatedResult
type PaginatedResult[T any] struct {
	PageNum  *int `json:"pageNum,omitempty"`
	PageSize *int `json:"pageSize,omitempty"`
	Total    int  `json:"total"`
	Data     []T  `json:"data"`
}

const defaultPageSize = 10

// CreateFromFullList 从完整列表创建分页结果
func CreateFromFullList[T any](list []T, query *CommonPageQuery) *PaginatedResult[T] {
	if list == nil {
		list = []T{}
	}
	result := &PaginatedResult[T]{Total: len(list)}
	result.Data = list
	if query != nil && query.PaginationEnabled() {
		pageNum := 1
		if query.PageNum != nil && *query.PageNum > 1 {
			pageNum = *query.PageNum
		}
		pageSize := defaultPageSize
		if query.PageSize != nil && *query.PageSize > 0 {
			pageSize = *query.PageSize
		}
		start := (pageNum - 1) * pageSize
		if start >= len(list) {
			start = len(list)
		}
		to := start + pageSize
		if to > len(list) {
			to = len(list)
		}
		result.Data = list[start:to]
		pn, ps := pageNum, pageSize
		result.PageNum = &pn
		result.PageSize = &ps
	}
	return result
}

// VersionedDto 对应 Java 的 VersionedDto
type VersionedDto interface {
	GetVersion() *string
	SetVersion(v *string)
}
