package req

// 分页默认配置
const (
	DefaultPage  = 1
	DefaultSize  = 50
	MaxSizeLimit = 1000 // 默认最大分页大小，防止内存耗尽攻击
	MinPage      = 1
	MinSize      = 1
)

type IDReq[T uint64 | string] struct {
	ID T `json:"id" yaml:"id" form:"id" binding:"required"`
}

type NameReq struct {
	Name string `form:"name" json:"name" binding:"required"`
}

type PaginationReq struct {
	Page int `json:"page" form:"page"`
	Size int `json:"size" form:"size"`
}

type LikeReq struct {
	Keyword string `json:"keyword" form:"keyword"`
}

type Captcha struct {
	UUID string `json:"uuid" form:"uuid" binding:"required"`
	Code string `json:"code" form:"code" binding:"required"`
}

// normalize 标准化分页参数，确保在合理范围内
func (p *PaginationReq) normalize(sizeLimit int) {
	if p.Page < MinPage {
		p.Page = DefaultPage
	}
	if p.Size < MinSize {
		p.Size = DefaultSize
	}
	if sizeLimit > 0 && p.Size > sizeLimit {
		p.Size = sizeLimit
	}
}

// FindPage 返回分页的 offset 和 limit
// sizeLimit 可选，指定最大分页大小，默认使用 MaxSizeLimit
func (p *PaginationReq) FindPage(sizeLimit ...int) (offset int, limit int) {
	maxSize := MaxSizeLimit
	if len(sizeLimit) > 0 && sizeLimit[0] > 0 {
		maxSize = sizeLimit[0]
	}
	p.normalize(maxSize)
	return (p.Page - 1) * p.Size, p.Size
}

// StartEnd 返回分页的 start 和 end 索引
func (p *PaginationReq) StartEnd() (start, end int) {
	p.normalize(MaxSizeLimit)
	start = (p.Page - 1) * p.Size
	end = p.Size + start
	return
}
