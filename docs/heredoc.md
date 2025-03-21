### Overview

1. **Read lines** one by one.
2. If a line begins with `#`, it’s a **Comment**.
4. If a line begins with `:` it’s a **CommandBlock** statement.
5. Within a **CommandBlock**:
   - Optionally parse a **Payload** on the same line (either single-line or `"<HEREDOC"` style).
   - Then parse zero or more **ParameterBlock** lines. Each starts with `$` plus a parameter name and may include a single-line or heredoc payload.
6. **Heredoc** parsing continues until encountering a line with exactly `HEREDOC`.

This abstract grammar ensures each command or plan is well-defined, each parameter is associated to exactly one command, and multi-line text (heredoc) is clearly delimited.

Below is one possible **abstract grammar**—in a pseudo-EBNF style—that captures the essential structure of the “command + parameters + heredoc” protocol. It intentionally ignores any domain-specific commands or parameter names (like `modify`, `rewrite`, `$search`, `$replace`) and focuses purely on the syntactic rules.

---

## Grammar

```
ProtocolFile       ::= (CommandBlock | Comment)*

Comment            ::= "#" [^ \n]* ( "\n" | EOF )

CommandBlock       ::= ":" CommandName Payload? (Comment | ParameterBlock)*

CommandName        ::= 1*(ALPHANUMERIC)

ParameterBlock     ::= "$" ParameterName Payload?

ParameterName      ::= 1*(ALPHANUMERIC)

Payload            ::= SingleLinePayload
                    | HeredocPayload

SingleLinePayload  ::= " " NO_EOL_TEXT "\n"?
                     ; (Meaning optional text after a space, up to line-end)

HeredocPayload     ::= "<HEREDOC" "\n" 
                       HEREDOC_BODY 
                       "\nHEREDOC" ("\n" | EOF)

HEREDOC_BODY       ::= (ANY_LINE_NOT_EQUAL_TO_SOLE_HEREDOC_LINE)*

; ------------------------------------------------------------
; Lexical notes (informal)
; ------------------------------------------------------------
; - ALPHANUMERIC means any combination of letters or digits (A-Z, a-z, 0-9). 
;   (Some grammars may also allow underscores, but it's omitted here for brevity.)
; - NO_EOL_TEXT means any text that does not include a newline.
; - ANY_LINE_NOT_EQUAL_TO_SOLE_HEREDOC_LINE means any line of text
;   that is not exactly "HEREDOC" on a line by itself.
; - Comments (leading "#") can appear anywhere outside a heredoc body, 
;   including between parameters or after the command line.
; - The optional newline in SingleLinePayload ensures we can 
;   handle either a trailing newline or end-of-file.
```

### Explanation

1. **ProtocolFile**  
   The entire file is a sequence of statements and comments. A “statement” can be either  or a `CommandBlock` (i.e., `:someCommand`).

2. **Comments**  
   Any line that begins with `#` (with no preceding whitespace) is treated as a comment. The comment continues until the end of that line.

4. **CommandBlock**  
   A generic command line starting with `:` plus a **CommandName**, optionally followed by a **Payload**, then zero or more **ParameterBlock** entries.  
   - The command name can be something like `modify`, `rewrite`, `create`, etc., but the grammar simply captures it as a token here.

5. **ParameterBlock**  
   Each parameter line starts with `$` plus a **ParameterName**, optionally followed by a **Payload**. This corresponds to constructs like  
   ```
   $search<HEREDOC
   ...
   HEREDOC
   ```  
   or  
   ```
   $description Some short explanation
   ```  

6. **Payload**  
   This can be either:
   - **SingleLinePayload**: a space followed by text up to the end of the line, or  
   - **HeredocPayload**: a multi-line block enclosed between  
     ```
     <HEREDOC
     ...
     HEREDOC
     ```  
     with the closing `HEREDOC` on its own line.

7. **HeredocPayload / HEREDOC_BODY**  
   The heredoc ends when we encounter a line whose *only* content is `HEREDOC`. Everything in between is captured verbatim as the heredoc body (including possible empty lines).  

8. **Lexical / Tokens**  
   The grammar uses some tokens like `ALPHANUMERIC_OR_SYMBOL_EXCEPT_$COLON`, which means any characters allowed in a command or parameter name except for `$` or `:`. Similarly, `NO_EOL_TEXT` is the text up to the end of the line, excluding newlines.


### Implementation Note on “HEREDOC”

Although this grammar literally uses the string `HEREDOC` in `HeredocPayload` to delimit the body, in practice **the `HEREDOC` token can be replaced by any unique marker**, such as `EOF` or `MYDOC`.  

The essential requirement is that the same marker is used on the opening line after `<` and again on its own line when closing.  

For example, you could do:

```
$content<MYDOC
This is a multi-line
payload for a heredoc.
MYDOC
```  

The parse logic remains the same, so long as the strings match exactly.