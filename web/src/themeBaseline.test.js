import fs from 'fs';
import path from 'path';

const readLocalFile = (file) => fs.readFileSync(path.resolve(__dirname, file), 'utf8');

describe('theme baseline contracts', () => {
  it('loads semantic css before project index css', () => {
    const source = readLocalFile('index.js');
    const semanticImportPos = source.indexOf("import 'semantic-ui-css/semantic.min.css';");
    const indexCssImportPos = source.indexOf("import './index.css';");

    expect(semanticImportPos).toBeGreaterThan(-1);
    expect(indexCssImportPos).toBeGreaterThan(-1);
    expect(semanticImportPos).toBeLessThan(indexCssImportPos);
  });

  it('defines required semantic tokens and shared layout classes', () => {
    const css = readLocalFile('index.css');
    ['--text-tertiary', '--success-color', '--warning-color', '--danger-color'].forEach((token) => {
      expect(css).toContain(`${token}:`);
    });
    ['.table-footer', '.table-pagination', '.page-shell', '.page-title'].forEach((className) => {
      expect(css).toContain(className);
    });
  });
});
