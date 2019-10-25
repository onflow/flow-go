const visit = require('unist-util-visit')
const vsctm = require('vscode-textmate')
const fs = require('fs')

class Highlighter {

    static async loadGrammar(path) {
        const rawGrammar = await fs.promises.readFile(path, 'utf-8')
        return vsctm.parseRawGrammar(rawGrammar.toString(), path)
    }

    static async loadTheme(path) {
        const rawTheme = await fs.promises.readFile(path, 'utf-8')
        return JSON.parse(rawTheme)
    }

    static async fromOptions({languageScopes, grammarPaths, themePath}) {
        const registry = new vsctm.Registry()

        for (let grammarPath of grammarPaths) {
            const grammar = await Highlighter.loadGrammar(grammarPath)
            await registry.addGrammar(grammar)
        }

        const theme = await Highlighter.loadTheme(themePath)
        registry.setTheme(theme)

        return new Highlighter(registry, languageScopes)
    }

    constructor(registry, languageScopes) {
        this.registry = registry
        this.languageScopes = languageScopes
    }

    // StackElementMetadata isn't exported by vscode-textmate
    static getForeground (metadata) {
        return (metadata & 8372224 /* FOREGROUND_MASK */) >>> 14 /* FOREGROUND_OFFSET */
    }

    async getLanguageGrammar(language) {
        const scopeName = this.languageScopes[language]
        if (scopeName === undefined)
            return
        return await this.registry.grammarForScopeName(scopeName)
    }

    highlight(code, grammar) {
        const colorMap = this.registry.getColorMap()

        const lines = code.split(/\r\n|\r|\n/)

        let ruleStack = null
        let result = []

        for (let i = 0, len = lines.length; i < len; i++) {
            const line = lines[i]
            // NOTE: only works properly when a theme is registered,
            // otherwise the tokens are merged because they have the same style
            const lineTokens = grammar.tokenizeLine2(line, ruleStack)
            const tokensLength = lineTokens.tokens.length / 2
            for (let j = 0; j < tokensLength; j++) {
                const startIndex = lineTokens.tokens[2 * j]
                const nextStartIndex = j + 1 < tokensLength ? lineTokens.tokens[2 * j + 2] : line.length
                const content = line.substring(startIndex, nextStartIndex)
                if (content === '') {
                    continue
                }

                const metadata = lineTokens.tokens[2 * j + 1]
                const foreground = Highlighter.getForeground(metadata)
                const color = colorMap[foreground]

                result.push({
                    type: 'element',
                    tagName: 'span',
                    properties: {style: `color: ${color}`},
                    children: [{type: 'text', value: content}]
                })
            }
            ruleStack = lineTokens.ruleStack

            result.push({
                type: 'element',
                tagName: 'span',
                children: [{type: 'text', value: '\n'}]
            })
        }

        return result
    }
}

function attacher(options) {
    const highlighterPromise =
        Highlighter.fromOptions(options)

    return async (ast) => {
        const highlighter = await highlighterPromise

        async function visitor(node) {
            const language = node.lang.split(',')[0]

            const grammar = await highlighter.getLanguageGrammar(language)
            if (!grammar) {
                throw new Error('Failed to load language grammar')
            }

            const highlighted =
                highlighter.highlight(node.value, grammar)

            node.data = {hChildren: highlighted}
        }

        return await visit(ast, 'code', visitor)
    }
}

module.exports = attacher
