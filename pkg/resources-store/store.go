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

var StoreObj *Store

// 监测哪个表存放的是最新的数据
type writeFlag int

const (
	writeM   writeFlag = 0
	writeMbk writeFlag = 1
)

// 这是一个写多读少的场景
// 所有资源存放的结构
// 读时：
// http --> GVR ---> storeKey(如果有需求的话，可以用更硬的算 key 函数) ---> ns/name ---> obj(json format)
// 写时：
// RuntimeObj + GVR ---> storeKey, entryKey, entryValue ---> entry ---> store
// 这里为什么用两个表呢？
// 考虑是在集群的资源有变化时，可以响应出来，增加资源一个表可以体现，如果一个资源少了，一直往一个表放就无法体现，这里用每次全量获取放入另一个表的方法来响应
type Store struct {
	mu, mbku sync.RWMutex
	m        map[storeKey]*entry
	mbk      map[storeKey]*entry
	flag     writeFlag
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

func init() {
	StoreObj = NewStore()
}

func NewStore() *Store {
	return &Store{
		m:    make(map[storeKey]*entry),
		mbk:  make(map[storeKey]*entry),
		flag: writeMbk,
	}
}

func newEntry() *entry {
	return &entry{
		m: make(map[entryKey]entryValue),
	}
}

func (store *Store) Add(gvr schema.GroupVersionResource, objs []runtime.Object) {
	// 如果上次写的是 mbk 表，则写入 m 表
	if store.flag == writeMbk {
		store.addm(gvr, objs)
	} else if store.flag == writeM {
		store.addmbk(gvr, objs)
	}
}

func (store *Store) addm(gvr schema.GroupVersionResource, objs []runtime.Object) {
	store.mu.Lock()
	defer store.mu.Unlock()

	key := storeKeyFunc(gvr)

	entryObj, ok := store.m[key]

	if ok {
		entryObj.add(objs)
		// store.flag = writeM
		store.mbk = make(map[storeKey]*entry)
		return
	}
	entryObj = newEntry()
	entryObj.add(objs)
	store.m[key] = entryObj
	// store.flag = writeM
	store.mbk = make(map[storeKey]*entry)
}

func (store *Store) addmbk(gvr schema.GroupVersionResource, objs []runtime.Object) {
	store.mbku.Lock()
	defer store.mbku.Unlock()

	key := storeKeyFunc(gvr)

	entryObj, ok := store.mbk[key]

	if ok {
		entryObj.add(objs)
		// store.flag = writeMbk
		store.m = make(map[storeKey]*entry)
		return
	}
	entryObj = newEntry()
	entryObj.add(objs)
	store.mbk[key] = entryObj
	// store.flag = writeMbk
	store.m = make(map[storeKey]*entry)
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

	var entryObj *entry
	var ok bool
	if store.flag == writeM {
		entryObj, ok = store.m[storeKey]
		if !ok {
			return "", ok
		}
	} else if store.flag == writeMbk {
		entryObj, ok = store.mbk[storeKey]
		if !ok {
			return "", ok
		}
	}

	// entryObj, ok := store.m[storeKey]
	// if !ok {
	// 	return "", ok
	// }

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

	var entryObj *entry
	var ok bool
	if store.flag == writeM {
		entryObj, ok = store.m[key]
		if !ok {
			return nil
		}
	} else if store.flag == writeMbk {
		entryObj, ok = store.mbk[key]
		if !ok {
			return nil
		}
	}

	// entryObj, ok := store.m[key]
	// if !ok {
	// 	return nil
	// }

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

	var entryObj *entry
	var ok bool
	if store.flag == writeM {
		entryObj, ok = store.m[storeKey]
		if !ok {
			return nil
		}
	} else if store.flag == writeMbk {
		entryObj, ok = store.mbk[storeKey]
		if !ok {
			return nil
		}
	}

	// entryObj, ok := store.m[storeKey]
	// if !ok {
	// 	return nil
	// }

	strs := entryObj.list(ns)

	return strs
}

func (e *entry) list(ns string) []string {
	strs := make([]string, 0)

	for entryObjKey, entryObjValue := range e.m {
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
