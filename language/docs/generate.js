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

const highlight = require('./highlight')

const remark2retext = require('remark-retext')
const english = require('retext-english')
const indefiniteArticle = require('retext-indefinite-article')
const repeatedWords = require('retext-repeated-words')

const remark2rehype = require('remark-rehype')
const doc = require('rehype-document')
const format = require('rehype-format')
const html = require('rehype-stringify')
const addClasses = require('rehype-add-classes')

const puppeteer = require('puppeteer')
const path = require('path')

const processor = unified()
  .use(markdown)
  .use(toc)
  .use(slug)
  .use(autolink)
  .use(validateLinks)
  .use(styleGuide)
  .use(highlight, {
    languageScopes: {'bamboo': 'source.bamboo'},
    grammarPaths: ['../tools/vscode-extension/syntaxes/bamboo.tmGrammar.json'],
    themePath: './light_vs.json'
  })
  .use(
    remark2retext,
    unified()
      .use(english)
      .use(indefiniteArticle)
      .use(repeatedWords)
  )
  .use(sectionize)
  .use(remark2rehype)
  .use(doc, {
    title: 'Bamboo Programming Language',
    css: ['style.css', "https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/3.0.1/github-markdown.css"]
  })
  .use(addClasses, {
    body: 'markdown-body'
  })
  .use(format)
  .use(html)

async function writeHTML(file) {
  file.extname = '.html'
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

processor.process(vfile.readSync('language.md'), async (err, file) => {
  if (err)
    throw err;
  console.error(report(file))
  await writeHTML(file)
  await writePDF(file)
})
