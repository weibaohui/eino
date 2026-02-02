package schema

// receiveN 使用 switch-case select 高效地从选定的流中接收数据。
// 它最多支持 maxSelectNum 个流。
// 参数：
//   - chosenList: 需要从中接收数据的流的索引列表。
//   - ss: 所有的流列表。
//
// 返回值：
//   - index: 接收到数据的流的索引。
//   - item: 接收到的数据项。
//   - ok: 通道是否仍然打开。
func receiveN[T any](chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
	return []func(chosenList []int, ss []*stream[T]) (index int, item *streamItem[T], ok bool){
		nil,
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			item, ok := <-ss[chosenList[0]].items
			return chosenList[0], &item, ok
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			case item, ok := <-ss[chosenList[3]].items:
				return chosenList[3], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			case item, ok := <-ss[chosenList[3]].items:
				return chosenList[3], &item, ok
			case item, ok := <-ss[chosenList[4]].items:
				return chosenList[4], &item, ok
			}
		},
	}[len(chosenList)](chosenList, ss)
}
