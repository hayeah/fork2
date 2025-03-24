+++
--select cmd/vibe/askgo 
--copy
+++

from: d73eaad

- vibe: change `diff` flag to `role`
	- remove `--diff`
	- add the "--role" flag
		- default to coder
	- when rendering in handleOutput, wrap the specified role in "<...>"
- fix tests
