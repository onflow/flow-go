const getStdin = require('get-stdin')
const { Highlighter } = require('./highlight')
const toHtml = require('hast-util-to-html')

const makeHighlightOptions = (target) => ({
    languageScopes: {'cadence': 'source.cadence'},
    grammarPaths: ['../tools/vscode-extension/syntaxes/cadence.tmGrammar.json'],
    themePath: './light_vs.json',
    target: target,
})

module.exports = { makeHighlightOptions }

if (require.main === module) {
    (async () => {
        const code = await getStdin()

        const highlighter =
            await Highlighter.fromOptions(makeHighlightOptions('html'))

        const grammar = await highlighter.getLanguageGrammar('cadence')
        if (!grammar) {
            throw new Error('Failed to load language grammar')
        }

        const highlighted =
            highlighter.highlight(code, grammar)

        console.log(toHtml(
            {
                type: "element",
                tagName: "code",
                children: [
                    {
                        type: "element",
                        tagName: "pre",
                        children: highlighted,
                    }
                ],
            }
        ))
    })()
}
