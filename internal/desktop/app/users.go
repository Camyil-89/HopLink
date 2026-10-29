package desktop

import (
	"fmt"
	"regexp"
)

func (a *App) SaveUsers(users []User) (AppConfig, error) {
	seen := map[string]bool{}
	for i := range users {
		u := &users[i]
		if u.Active == nil {
			u.Active = boolPointer(true)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,48}$`).MatchString(u.Name) {
			return a.GetState(), fmt.Errorf("имя пользователя: латиница, цифры, '.', '_' и '-' (до 48 символов)")
		}
		if u.Password == "" {
			return a.GetState(), fmt.Errorf("пароль пользователя не может быть пустым")
		}
		if seen[u.Name] || seen["p:"+u.Password] {
			return a.GetState(), fmt.Errorf("имена и пароли должны быть уникальными")
		}
		seen[u.Name] = true
		seen["p:"+u.Password] = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	links := a.links
	for i := range users {
		if !users[i].isActive() {
			users[i].Link = ""
			delete(links, users[i].Name)
			continue
		}
		if users[i].Link == "" {
			users[i].Link = links[users[i].Name]
		}
	}
	a.config.Users = users
	return a.config, a.saveLocked()
}
func (a *App) clearUserLinksLocked() {
	a.links = map[string]string{}
	for i := range a.config.Users {
		a.config.Users[i].Link = ""
	}
}
