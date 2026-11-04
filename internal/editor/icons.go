package editor

import "github.com/egoist/mygo/ui"

// The icons of the toolbar, in the colors it draws them.
var (
	IconRect    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="5" width="16" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconEllipse = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><ellipse cx="12" cy="12" rx="8.5" ry="6.5" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconArrow   = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M4 20 L16 8" fill="none" stroke="currentColor" stroke-width="2"/><path d="M12 4 L20 4 L20 12 Z" fill="currentColor"/></svg>`)
	IconPen     = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M3 18 C6 10 8 20 11 13 C14 6 17 16 21 8" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconText    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M4 5 H20 V9 H17 V7 H13.5 V19 H16 V21 H8 V19 H10.5 V7 H7 V9 H4 Z" fill="currentColor"/></svg>`)
	IconMosaic  = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" fill="currentColor"/><rect x="13" y="4" width="7" height="7" fill="currentColor" opacity="0.5"/><rect x="4" y="13" width="7" height="7" fill="currentColor" opacity="0.5"/><rect x="13" y="13" width="7" height="7" fill="currentColor"/></svg>`)
	IconBadge   = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" fill="none" stroke="currentColor" stroke-width="2"/><path d="M10 10.5 L12.5 8.5 V16" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconCrop    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M7 3 V17 A2 2 0 0 0 9 19 H21" fill="none" stroke="currentColor" stroke-width="2"/><path d="M3 7 H15 A2 2 0 0 1 17 9 V21" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconUndo    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M9 7 L4 12 L9 17" fill="none" stroke="currentColor" stroke-width="2"/><path d="M4 12 H14 A5 5 0 0 1 14 22" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconRedo    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M15 7 L20 12 L15 17" fill="none" stroke="currentColor" stroke-width="2"/><path d="M20 12 H10 A5 5 0 0 0 10 22" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconCancel  = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6 6 L18 18 M18 6 L6 18" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
	IconSave    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M5 13 L10 18 L19 6" fill="none" stroke="currentColor" stroke-width="2.4"/></svg>`)
	IconCopy    = svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="8" width="12" height="12" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><path d="M8 8 V6 A2 2 0 0 1 10 4 H18 A2 2 0 0 1 20 6 V14 A2 2 0 0 1 18 16 H16" fill="none" stroke="currentColor" stroke-width="2"/></svg>`)
)

// svg parses one of the toolbar's icons, which are written here and known to
// parse.
func svg(markup string) *ui.SVG { return ui.MustParseSVG([]byte(markup)) }
