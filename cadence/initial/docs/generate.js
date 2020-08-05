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

const unified = require('unified')
const vfile = require('to-vfile')
const report = require('vfile-reporter')

const markdown = require('remark-parse')
const toc = require('remark-toc')
const slug = require('remark-slug')
const autolink = require('remark-autolink-headings')
const styleGuide = require('remark-preset-lint-markdown-style-guide')
const validateLinks = require('remark-validate-links')
const sectionize = require('remark-sectionize')

const { Highlighter } = require('./highlight')

const remark2retext = require('remark-retext')
const english = require('retext-english')
const indefiniteArticle = require('retext-indefinite-article')
const repeatedWords = require('retext-repeated-words')

const stringify = require('remark-stringify')

const remark2rehype = require('remark-rehype')
const doc = require('rehype-document')
const format = require('rehype-format')
const html = require('rehype-stringify')
const addClasses = require('rehype-add-classes')

const puppeteer = require('puppeteer')
const path = require('path')

const { makeHighlightOptions } = require('./highlight-cadence')


const toHtml = require('hast-util-to-html')

const visit = require('unist-util-visit')


function highlight(options) {
  const highlighterPromise =
      Highlighter.fromOptions(options)

  return async (ast) => {
    const highlighter = await highlighterPromise

    async function visitor(node) {
      if (!node.lang) {
        throw new Error('Missing language tag at line ' + node.position.start.line)
      }

      const language = node.lang.split(',')[0]

      const grammar = await highlighter.getLanguageGrammar(language)
      if (!grammar) {
        throw new Error('Failed to load language grammar')
      }

      const highlighted =
          highlighter.highlight(node.value, grammar)

      switch (options.target) {
          // 'html' adds the highlighted code to the remark node, for use with rehype
        case 'html':
          node.data = {hChildren: highlighted}
          break
          // 'markdown' replaces the remark code fence node with an HTML node of the highlighted code
        case 'markdown':
          node.type = 'html'
          node.value = toHtml(
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
          )
          break
      }
    }

    return await visit(ast, 'code', visitor)
  }
}




// - target:
//   - 'html': generate HTML
//   - 'markdown': generate Markdown
function buildPipeline(target) {

  const base = unified()
    .use(markdown)
    .use(toc)
    .use(slug)
    .use(autolink)
    .use(validateLinks)
    .use({
      plugins: styleGuide.plugins.filter(elem => {
        if (!Array.isArray(elem))
          return elem;
        return elem[0].displayName !== 'remark-lint:list-item-indent'
      })
    })
    .use(highlight, makeHighlightOptions(target))
    .use(
      remark2retext,
      unified()
        .use(english)
        .use(indefiniteArticle)
        .use(repeatedWords)
    )

  switch (target) {
  case 'html':
    return base
      .use(sectionize)
      .use(remark2rehype)
      .use(doc, {
        title: 'Cadence Programming Language',
        css: ['style.css', "https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/3.0.1/github-markdown.css"]
      })
      .use(addClasses, {
        body: 'markdown-body'
      })
      .use(format)
      .use(html)

  case 'markdown':
    return base
      .use(stringify, {
        entities: 'escape'
      })
  }
}

async function writeHTML(file) {
  file.extname = '.html'
  await vfile.write(file)
}

async function writeMarkdown(file) {
  file.extname = '.md'
  file.stem += '.generated'
  await vfile.write(file)
}


async function writePDF(file) {
  file.extname = '.html'
  const browser = await puppeteer.launch({
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--font-render-hinting=medium'
    ]
  })
  const page = await browser.newPage()
  const url = `file:${path.join(__dirname, file.path)}`
  await page.goto(url, {waitUntil: 'networkidle0'});
  // await page.setContent(String(file), {waitUntil: 'networkidle0'})
  await page.emulateMedia('print');
  file.extname = '.pdf'
  await page.pdf({
    path: file.path,
    printBackground: true,
    preferCSSPageSize: true
  })
  await browser.close()
}

buildPipeline('html').process(vfile.readSync('language.md'), async (err, file) => {
  if (err)
    throw err;
  console.error(report(file))
  await writeHTML(file)
  await writePDF(file)
})

buildPipeline('markdown').process(vfile.readSync('language.md'), async (err, file) => {
  if (err)
    throw err;
  console.error(report(file))
  await writeMarkdown(file)
})
