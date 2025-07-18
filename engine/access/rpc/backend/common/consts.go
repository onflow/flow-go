package common

// DefaultLoggedScriptsCacheSize is the default size of the lookup cache used to dedupe logs of scripts sent to ENs
// limiting cache size to 16MB and does not affect script execution, only for keeping logs tidy
const DefaultLoggedScriptsCacheSize = 1_000_000
