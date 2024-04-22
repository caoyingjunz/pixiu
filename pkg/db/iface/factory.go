package iface

type ShareDaoFactory interface {
	Cluster() ClusterInterface
	Tenant() TenantInterface
	User() UserInterface
}
