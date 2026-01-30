* Prefer value semantics over pointer semantics if feasible
* Have either pointer or value receivers, not both, for methods on a type
* Have constants, global variables, and type definitions always at the top of the file, before functions and methods
* Have public functions and method before private ones in the file.
* constructors must be always the first functions in a file (before all the methods), immediately after type definitions. even if they're non-public.
* Binary is in ./cmd/NAME/main.go
* Main file should be fairly small and only be concerned about argument/flags parsing and calling functions from the internal package
* Internal code is in ./internal
* Version of the app is a constent in the ./internal/version.go and a -version flag (main.go) prints it out.
* Avoid using package-level variables unless absolutely necessary; prefer dependency injection
* Use context.Context as the first parameter for functions that may block, perform I/O, or be canceled
* Use error wrapping (fmt.Errorf with %w) to provide context for errors
* Prefer explicit interface satisfaction (var _ MyInterface = (*MyType)(nil)) for public types
* Keep interfaces small and focused; accept interfaces, return concrete types
* Use gofmt and goimports to enforce formatting and import order
* Document all exported identifiers with comments starting with the identifier's name
* Avoid stutter in package and type names (e.g., "foo.FooType" should be just "foo.Type")
* Use short variable names for short-lived variables, longer names for longer-lived ones
* Use iota for related constant values
* Use table-driven tests for unit testing
* Avoid using panic except for truly unrecoverable errors (e.g., programmer errors)
* Use defer to close resources (files, connections) as soon as they are opened
* Avoid large functions, split them into smaller, focused helper functions. 50 lines per function max.
* Aim for a unit test coverage of 60%
* Avoid code duplication where reasonable.
