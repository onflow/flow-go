/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
