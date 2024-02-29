package cache

// node represents a node in a queue.
// The value of the node is supposed to be the item, and the item and the node should have cyclic relationship.
type node[T any] struct {
	next  *node[T]
	prev  *node[T]
	value T
}

func newNode[T any](value T) *node[T] {
	return &node[T]{value: value}
}

// queue is a data structure used to keep track of the least and most used items,
// along with how often they are promoted.
type queue[T any] struct {
	head *node[T]
	tail *node[T]
}

func newQueue[T any]() *queue[T] {
	return &queue[T]{}
}

func (q *queue[T]) pushToFront(value T) *node[T] {
	n := newNode(value)
	if q.head == nil {
		q.head = n
		q.tail = n
		return n
	}
	n.next = q.head
	q.head.prev = n
	q.head = n
	return n
}

func (q *queue[T]) remove(node *node[T]) {
	if node == nil {
		return
	}
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		q.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		q.tail = node.prev
	}

	node.next = nil
	node.prev = nil
}

func (q *queue[T]) moveToFront(node *node[T]) {
	if node == nil || q.head == node {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	} else {
		q.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		q.tail = node.prev
	}

	node.next = q.head
	node.prev = nil
	if q.head != nil {
		q.head.prev = node
	}
	q.head = node
	if q.tail == nil {
		q.tail = node
	}
}
