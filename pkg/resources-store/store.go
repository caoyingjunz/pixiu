package resourcesstore

import (
	"encoding/json"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// 这是一个写多读少的场景，需要设计出读写效率较高的实现
// 所有资源存放的结构
// 读时：
// http --> GVR ---> storeKey(如果有需求的话，可以用更硬的算 key 函数) ---> ns/name ---> obj(json format)
// 写时：
// RuntimeObj + GVR ---> storeKey, entryKey, entryValue ---> entry ---> store
// TODO: 后续再增加个表，保存一个周期的起始数据和最新数据，然后做同步，写永远往一个里写，读需要设计下往哪读
type Store struct {
	mu sync.RWMutex
	m  map[storeKey]*entry
}

// 每种 gvr 对应一个 entry
// runtime.Object 是指针类型的数据
type entry struct {
	// mu sync.RWMutex
	m map[entryKey]entryValue
}

type storeKey string

type entryKey string

type entryValue string

var StoreObj *Store

func init() {
	StoreObj = NewStore()
}

func NewStore() *Store {
	return &Store{
		m: make(map[storeKey]*entry),
	}
}

func newEntry() *entry {
	return &entry{
		m: make(map[entryKey]entryValue),
	}
}

func (store *Store) Add(gvr schema.GroupVersionResource, objs []runtime.Object) {
	store.mu.Lock()
	defer store.mu.Unlock()

	key := storeKeyFunc(gvr)

	entry, ok := store.m[key]

	if ok {
		entry.add(objs)
		return
	}
	entry = newEntry()
	entry.add(objs)
	store.m[key] = entry
}

// 将 kubernetes 的资源存放进 entry
// 由于资源是一直变化的，所以这里直接进行重写操作，不检查原本是否存在
func (e *entry) add(objs []runtime.Object) {
	// e.mu.Lock()
	// defer e.mu.Unlock()

	for _, obj := range objs {
		key, err := entryKeyFunc(obj)
		if err != nil {
			klog.Errorf("get entry key failed, obj: %v, err: %v", obj, err)
			continue
		}
		value, err := entryValueFunc(obj)
		if err != nil {
			klog.Errorf("get entry value failed, obj: %v, err: %v", obj, err)
			continue
		}

		e.m[key] = value
	}
}

func storeKeyFunc(gvr schema.GroupVersionResource) storeKey {
	// fmt.Println(gvr.String())
	return storeKey(gvr.String())
}

func entryKeyFunc(obj runtime.Object) (entryKey, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("get entry key failed, obj: %v, err: %v", obj, err)
		return "", nil
	}

	return entryKey(key), nil
}

func entryValueFunc(obj runtime.Object) (entryValue, error) {
	byteData, err := json.Marshal(obj)
	if err != nil {
		klog.Errorf("get entry value failed, obj: %v, err: %v", obj, err)
		return "", err
	}

	return entryValue(string(byteData)), nil
}

func (store *Store) GetByNamespaceAndName(gvr schema.GroupVersionResource, ns, name string) (entryValue, bool) {
	storeKey := storeKeyFunc(gvr)

	store.mu.RLock()
	defer store.mu.RUnlock()

	entryObj, ok := store.m[storeKey]
	if !ok {
		return "", ok
	}

	entryObjValue, ok := entryObj.get(ns, name)

	return entryObjValue, ok
}

func (e *entry) get(ns, name string) (entryValue, bool) {
	var entryObjKey string
	if ns == "" {
		entryObjKey = name
	} else {
		entryObjKey = ns + "/" + name
	}

	entryObjValue, ok := e.m[entryKey(entryObjKey)]

	return entryObjValue, ok
}

// 不区分 ns 的获取资源
func (store *Store) ListAll(gvr schema.GroupVersionResource) []string {
	key := storeKeyFunc(gvr)

	store.mu.RLock()
	defer store.mu.RUnlock()

	entryObj, ok := store.m[key]
	if !ok {
		return nil
	}

	strs := make([]string, len(entryObj.m))

	for _, entryObjValue := range entryObj.m {
		strs = append(strs, string(entryObjValue))
	}
	return strs
}

// 获取某个 namespace 下的某种资源
// eg: kubectl get pod -n default
func (store *Store) ListByNamespace(gvr schema.GroupVersionResource, ns string) []string {
	storeKey := storeKeyFunc(gvr)

	store.mu.RLock()
	defer store.mu.RUnlock()

	entryObj, ok := store.m[storeKey]
	if !ok {
		return nil
	}

	strs := make([]string, 0)

	for entryObjKey, entryObjValue := range entryObj.m {
		// 分割 entryObjKey
		keys := strings.Split(string(entryObjKey), "/")
		// 长度为 2 说明是区分 ns 的资源
		if len(keys) == 2 {
			keyNS := keys[0]
			if keyNS == ns {
				strs = append(strs, string(entryObjValue))
				continue
			}
		}
	}

	return strs
}
