//go:build !(js && wasm)

package main

// The interactive demo needs syscall/js, so it exists only in the GOOS=js
// build. Everywhere else -interactive is rejected up front rather than
// silently doing nothing.
const interactiveSupported = false

func serveInteractive() {}
