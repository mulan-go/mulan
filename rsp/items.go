package rsp

type Number interface {
	int | int64 | int32 | int16 | int8 | uint | uint64 | uint32 | uint16 | uint8
}

type ItemRsp = Rsp[*Items]

type Items struct {
	Items any   `json:"items"`
	Ext   any   `json:"ext,omitempty"`
	Total int64 `json:"total"`
}

func Item[T Number](total T, items any) ItemRsp {
	return ItemRsp{AppError: Success, Data: &Items{Items: items, Total: int64(total)}}
}
func ItemExt[T Number](total T, items any, ext any) ItemRsp {
	return ItemRsp{AppError: Success, Data: &Items{Items: items, Total: int64(total), Ext: ext}}
}

func ItemCode(err *AppError) ItemRsp {
	return ItemRsp{AppError: err, Data: &Items{Items: []struct{}{}, Total: 0}}
}

func ItemNone() ItemRsp { return ItemCode(ErrNotFound) }
