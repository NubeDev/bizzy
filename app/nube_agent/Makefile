.PHONY: get analyze run-linux run-web build-linux build-web build-apk build-ios serve clean

# Install dependencies
get:
	flutter pub get
	cd server && dart pub get

# Run analyzer
analyze:
	flutter analyze lib/

# ── Run ──

run-linux:
	flutter run -d linux

run-chrome:
	flutter run -d chrome

# ── Build ──

build-linux:
	flutter build linux

build-web:
	flutter build web
	rm -rf server/frontend
	cp -r build/web server/frontend

build-apk:
	flutter build apk

build-ios:
	flutter build ios

# ── Web server ──

# Build web then start the Dart proxy server
serve: build-web
	cd server && dart run bin/app_server.dart

# Start server without rebuilding (assumes build-web was already run)
serve-only:
	cd server && dart run bin/app_server.dart

# ── Clean ──

clean:
	flutter clean
	rm -rf server/frontend server/.dart_tool
