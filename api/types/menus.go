package types

type MenusReq struct {
	Status   int8   `json:"status"`    // 状态(1:启用 2:不启用)
	Memo     string `json:"memo"`      // 备注
	ParentID int64  `json:"parent_id"` // 父级ID
	URL      string `json:"url"`       // 菜单URL
	Name     string `json:"name"`      // 菜单名称
	Sequence int    `json:"sequence"`  // 排序值
	MenuType int8   `json:"menu_type"` // 菜单类型 1 左侧菜单,2 按钮, 3 非展示权限
	Icon     string `json:"icon"`      // icon
	Method   string `json:"method"`    // 操作类型 none/GET/POST/PUT/DELETE
	Code     string `json:"code"`      // 前端鉴权code 例： user:role:add, user:role:delete

}

type UpdateMenusReq struct {
	Status          int8   `json:"status"`    // 状态(1:启用 2:不启用)
	Memo            string `json:"memo"`      // 备注
	ParentID        int64  `json:"parent_id"` // 父级ID
	URL             string `json:"url"`       // 菜单URL
	Name            string `json:"name"`      // 菜单名称
	Sequence        int    `json:"sequence"`  // 排序值
	MenuType        int8   `json:"menu_type"` // 菜单类型 1 左侧菜单,2 按钮, 3 非展示权限
	Icon            string `json:"icon"`      // icon
	Method          string `json:"method"`    // 操作类型 none/GET/POST/PUT/DELETE
	Code            string `json:"code"`      // 前端鉴权code 例： user:role:add, user:role:delete
	ResourceVersion int64  `json:"resource_version"`
}
