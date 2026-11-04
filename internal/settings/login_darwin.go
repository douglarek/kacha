//go:build darwin

package settings

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// statusEnabled is SMAppServiceStatusEnabled.
const statusEnabled = 1

var (
	loginOnce sync.Once
	loginErr  error
	loginMsg  uintptr
	loginCls  func(name *byte) uintptr
	loginSel  func(name *byte) uintptr
	// loginKeep holds the C strings the runtime reads, which must stay put
	// for as long as the app runs.
	loginKeep [][]byte
)

// SetLaunchAtLogin opens the app when the user logs in, as a login item of
// macOS, which System Settings lists among the others with its icon, or
// takes that away.
func SetLaunchAtLogin(on bool) error {
	service, err := loginService()
	if err != nil {
		return err
	}
	pool := loginPool()
	defer loginPop(pool)
	var nsErr uintptr
	sel := "unregisterAndReturnError:"
	if on {
		sel = "registerAndReturnError:"
	}
	if loginSend(service, loginSelector(sel), uintptr(unsafe.Pointer(&nsErr))) == 0 {
		return fmt.Errorf("login item: %s", loginReason(nsErr))
	}
	return nil
}

// LaunchAtLogin reports whether the app opens at login now, as the user may
// have changed it in System Settings.
func LaunchAtLogin() bool {
	service, err := loginService()
	if err != nil {
		return false
	}
	pool := loginPool()
	defer loginPop(pool)
	return loginSend(service, loginSelector("status")) == statusEnabled
}

// loginService returns the login item of the app.
func loginService() (uintptr, error) {
	loginOnce.Do(loadLogin)
	if loginErr != nil {
		return 0, loginErr
	}
	class := loginCls(loginCString("SMAppService"))
	if class == 0 {
		return 0, errors.New("login item: ServiceManagement is not there, macOS 13 is needed")
	}
	service := loginSend(class, loginSelector("mainAppService"))
	if service == 0 {
		return 0, errors.New("login item: no main app service")
	}
	return service, nil
}

// loginReason writes what an NSError says about a refusal.
func loginReason(nsErr uintptr) string {
	if nsErr == 0 {
		return "refused"
	}
	pool := loginPool()
	defer loginPop(pool)
	code := loginSend(nsErr, loginSelector("code"))
	return fmt.Sprintf("refused with %d", int64(code))
}

// loadLogin opens ServiceManagement for the login item, once.
func loadLogin() {
	lib, err := purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement",
		purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		loginErr = err
		return
	}
	if loginMsg, err = purego.Dlsym(lib, "objc_msgSend"); err != nil {
		loginErr = err
		return
	}
	purego.RegisterLibFunc(&loginCls, lib, "objc_getClass")
	purego.RegisterLibFunc(&loginSel, lib, "sel_registerName")
}

// loginSend sends a message to an object.
func loginSend(obj, sel uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(loginMsg, append([]uintptr{obj, sel}, args...)...)
	return r
}

// loginSelector interns a selector.
func loginSelector(name string) uintptr { return loginSel(loginCString(name)) }

// loginPool opens an autorelease pool, which the caller pops.
func loginPool() uintptr {
	return loginSend(loginCls(loginCString("NSAutoreleasePool")), loginSelector("new"))
}

// loginPop closes an autorelease pool.
func loginPop(pool uintptr) {
	if pool != 0 {
		loginSend(pool, loginSelector("drain"))
	}
}

// loginCString returns s as a C string that stays for the life of the app.
func loginCString(s string) *byte {
	buf := append(make([]byte, 0, len(s)+1), s...)
	buf = append(buf, 0)
	loginKeep = append(loginKeep, buf)
	runtime.KeepAlive(&loginKeep[len(loginKeep)-1])
	return &loginKeep[len(loginKeep)-1][0]
}
