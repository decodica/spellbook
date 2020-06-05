package navigation

import (
	"context"
	"decodica.com/spellbook"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	"sort"
)

const keyMenuCache = "__page_menu_cache"

func PageId(locale string, url string) string {
	return locale + "-" + url
}

func TranslatedMenuItem(ctx context.Context, locale string, mi *MenuItem) *MenuItem {
	menu, err := GetMenu(ctx, locale, "")
	if err != nil {
		log.Errorf(ctx, "unable to retrieve translated menu for menu item %q %q", mi.Code, mi.Locale)
		return nil
	}
	for _, m := range menu {
		if m.Code == mi.Code {
			return &m
		}
	}
	return nil
}

func GetMenu(ctx context.Context, locale string, parent string) (Menu, error) {
	if !spellbook.Application().SupportsLocale(locale) {
		return nil, spellbook.NewUnsupportedError()
	}

	var menu map[string]Menu
	_, err := memcache.JSON.Get(ctx, keyMenuCache, &menu)

	if err == nil {
		// check if the language menu has been build
		m, ok := menu[locale]
		if ok {
			return m, nil
		}
	}

	// in every other case the menu has not been built for the given locale
	// so we rebuild it from storage
	menu, err = retrieveMenu(ctx)
	if err != nil {
		log.Errorf(ctx, "unable to retrieve menu for locale %q: %s", locale, err.Error())
		return nil, err
	}

	// save the menu in memcache
	i := memcache.Item{}
	i.Object = menu
	i.Key = keyMenuCache
	err = memcache.JSON.Set(ctx, &i)
	if err != nil {
		log.Errorf(ctx, "unable to save menu to memcache: %s", err.Error())
	}

	return menu[locale], nil
}

// clears the cached menu, thus forcing a subsequent menu rebuild
func InvalidateMenu(ctx context.Context) {
	if err := memcache.Delete(ctx, keyMenuCache); err != nil {
		log.Errorf(ctx, "unable to invalidate menu: %s", err.Error())
	}
}

func retrieveMenu(ctx context.Context) (map[string]Menu, error) {
	var menus []*Page
	q := model.NewQuery((*Page)(nil))
	if err := q.GetMulti(ctx, &menus); err != nil {
		return nil, err
	}

	// group the menu by locale
	menu := map[string]Menu{}
	for _, v := range menus {
		localized, _ := menu[v.Locale]
		m := NewMenuItemFromPage(v)
		localized = append(localized, m)
		menu[v.Locale] = localized
	}

	// order the menu by the order field
	for _, v := range menu {
		sort.Sort(MenuByOrder(v))
	}

	return menu, nil
}
