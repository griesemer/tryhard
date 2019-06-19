## tryhard command

`tryhard` is a very basic tool to list and rewrite `try` candidate statements.
It operates on a file-by-file basis and does not type-check the code;
potential statements are recognized and rewritten purely based on pattern
matching, with the very real possibility of false positives. Use utmost
caution when using the rewrite feature (-r flag) and have a backup as needed.

Given a file, it operates on that file; given a directory, it operates on all
`.go` files in that directory, recursively. Files starting with a period are ignored.

`tryhard` considers each top-level function with a last result of named type `error`,
which may or may not be the predefined type `error`. Inside these functions `tryhard`
considers assignments of the form

```Go
v1, v2, ..., vn, <err> = f() // can also be := instead of =
```

followed by an `if` statement of the form

```Go
if <err> != nil {
	return ..., <err>
}
```

or an `if` statement with an init expression matching the above assignment. The
error variable `<err>` must be called `err`, unless specified otherwise with the
-err flag; the variable may or may not be of type `error` or correspond to the
result error. The return statement must contain one or more return expressions,
with all but the last one denoting a zero value of sorts (a zero number literal,
an empty string, an empty composite literal, etc.). The last result must be the
variable `<err>`.

**CAUTION**: If the rewrite flag (-r) is specified, the file is updated in place!
         Make sure you can revert to the original.

Function literals (closures) are currently ignored.

### Usage:
```
tryhard [flags] [path ...]
```

The flags are:
```
-l
	list positions of potential `try` candidate statements
-r
	rewrite potential `try` candidate statements to use `try`
-err
	name of error variable; using "" permits any name (default "err")
```

If no -l is provided, `tryhard` reports the number of potential candidates found.
