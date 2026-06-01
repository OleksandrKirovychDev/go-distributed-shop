package domain

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

func ParseRole(raw string) (Role, error) {
	switch Role(raw) {
	case RoleCustomer:
		return RoleCustomer, nil
	case RoleAdmin:
		return RoleAdmin, nil
	default:
		return "", ErrInvalidRole
	}
}

func (r Role) String() string { return string(r) }
