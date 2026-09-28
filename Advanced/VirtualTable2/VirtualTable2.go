// 虚表排序
package main

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/twgh/xcgui/app"
	"github.com/twgh/xcgui/widget"
	"github.com/twgh/xcgui/window"
	"github.com/twgh/xcgui/xc"
	"github.com/twgh/xcgui/xcc"
)

var (
	list *widget.List

	// 虚表数据源
	listItems []*listItem
	// 排序状态 (0: 升序, 1: 降序, -1: 尚未排序过, 第一次点击表头不排序)
	sortState = -1
	// 保护数据访问的读写锁
	dataMutex sync.RWMutex
)

type listItem struct {
	Index  int
	Field1 string
	Field2 string
	Field3 string
}

func newListItem(index int) *listItem {
	num := strconv.Itoa(index + 1)
	return &listItem{
		Index:  index + 1,
		Field1: "行" + num + "--字段1",
		Field2: "行" + num + "--字段2",
		Field3: "行" + num + "--字段3",
	}
}

func main() {
	// 初始化界面库
	app.InitOrExit()
	a := app.New(true)
	// 启用自适应DPI
	a.EnableAutoDPI(true).EnableDPI(true)
	w := window.New(0, 0, 800, 600, "虚表排序示例", 0, xcc.Window_Style_Default)

	// 创建形状文本
	widget.NewShapeText(15, 35, 200, 30, "点击序号列表头进行排序", w.Handle)

	// 创建列表
	list = widget.NewList(15, 75, 770, 500, w.Handle)
	list.CreateAdapterHeader() // 创建表头数据适配器
	list.CreateAdapters(4)     // 创建数据适配器（4列）

	// 启用虚表模式
	list.EnableVirtualTable(true)

	// 添加表头
	list.AddColumnText2(120, "序号")
	list.AddColumnText2(150, "字段1")
	list.AddColumnText2(150, "字段2")
	list.AddColumnText2(150, "字段3")

	// 设置排序功能（第一列可排序）
	list.SetSort(0, 0, true)

	// 初始化数据（10000条）
	initListItems(10000)

	// 列表项模板创建完成事件
	list.Event_LIST_TEMP_CREATE_END1(onListTempCreateEnd)
	// 列表头项点击事件
	list.Event_LIST_HEADER_CLICK1(onListHeaderClick)

	w.Show(true)
	a.Run()
	a.Exit()
}

// 初始化虚表数据
func initListItems(count int) {
	dataMutex.Lock()
	defer dataMutex.Unlock()

	listItems = make([]*listItem, count)
	for i := 0; i < count; i++ {
		listItems[i] = newListItem(i)
	}

	// 设置虚表行数
	list.SetVirtualRowCount(int32(count))
}

// 列表项模板创建完成事件, 动态加载数据
func onListTempCreateEnd(hEle int, pItem *xc.List_Item_, nFlag int32, pbHandled *bool) int {
	index := pItem.Index
	dataMutex.RLock()
	defer dataMutex.RUnlock()

	if index < 0 || index >= int32(len(listItems)) {
		return 0
	}

	item := listItems[index]
	for col := int32(0); col < list.GetColumnCount(); col++ {
		hEle := list.GetTemplateObject(index, col, 1)
		if xc.XC_GetObjectType(hEle) == xcc.XC_SHAPE_TEXT {
			switch col {
			case 0:
				xc.XShapeText_SetText(hEle, strconv.Itoa(item.Index))
			case 1:
				xc.XShapeText_SetText(hEle, item.Field1)
			case 2:
				xc.XShapeText_SetText(hEle, item.Field2)
			case 3:
				xc.XShapeText_SetText(hEle, item.Field3)
			}
		}
	}
	return 0
}

// 列表头项点击事件, 处理排序
func onListHeaderClick(hEle int, iItem int32, pbHandled *bool) int {
	if iItem != 0 { // 只对第一列排序
		return 0
	}
	// 上一次排序还没完成时不再触发, 排序完成后 toggleSort 里会把它置回 "0"
	if list.GetProperty("正在排序") == "1" {
		return 0
	}

	dataMutex.Lock()
	if sortState == -1 { // 处理第一次点击表头, 第一次是不需要排序的, 本来就是升序
		sortState = 0
		dataMutex.Unlock()
		return 0
	}
	dataMutex.Unlock()

	list.SetProperty("正在排序", "1")
	go toggleSort() // 在协程里排序更好, 因数据量大
	return 0
}

// 切换排序状态
func toggleSort() {
	dataMutex.Lock()

	// 切换排序状态
	sortState = (sortState + 1) % 2
	fmt.Println("排序状态:", sortState)

	switch sortState {
	case 0: // 升序
		sort.Slice(listItems, func(i, j int) bool {
			return listItems[i].Index < listItems[j].Index
		})
	case 1: // 降序
		sort.Slice(listItems, func(i, j int) bool {
			return listItems[i].Index > listItems[j].Index
		})
	}
	// 这里必须先解锁再调用UI线程: XC_CallUT 是阻塞的, 它要等UI线程执行完回调才返回,
	// 而 RefreshData 在UI线程触发的列表项模板创建事件里需要对数据加读锁,
	// 如果这时还持有写锁, UI线程和本协程就会互相等待, 程序直接卡死.
	dataMutex.Unlock()

	xc.XC_CallUT(func() {
		// 刷新列表数据
		list.RefreshData().Redraw(false)
		list.SetProperty("正在排序", "0")
	})
}
