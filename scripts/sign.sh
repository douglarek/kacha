#!/bin/sh
# Prints the identity mygo signs the app with, and unlocks the keychain that
# holds it: the one this machine made, or "-", an ad hoc signature, where it
# has none, which is what a checkout without the keychain gets.
#
# The keychain and its password live outside the repository:
#
#	security create-keychain -p "$(cat "$PASS")" "$HOME/Library/Keychains/kacha-signing.keychain-db"
#	security import "$HOME/.kacha/signing/kacha.p12" -k "$KEYCHAIN" -T /usr/bin/codesign
#	security set-key-partition-list -S apple-tool:,apple: -k "$(cat "$PASS")" "$KEYCHAIN"
#
# A signing identity keeps the permission grants of macOS across builds, which
# an ad hoc signature loses with every one.
set -e
IDENTITY="Kacha Local Signing"
KEYCHAIN="$HOME/Library/Keychains/kacha-signing.keychain-db"
PASS="$HOME/.kacha/signing/keychain.pass"
if [ ! -f "$PASS" ] || [ ! -f "$KEYCHAIN" ]; then
	printf '%s\n' "-"
	exit 0
fi
security unlock-keychain -p "$(cat "$PASS")" "$KEYCHAIN" >/dev/null
security list-keychains -d user | grep -q kacha-signing ||
	security list-keychains -d user -s login.keychain-db "$KEYCHAIN"
printf '%s\n' "$IDENTITY"
