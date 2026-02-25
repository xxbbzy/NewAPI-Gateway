import fs from 'fs';
import path from 'path';

const readFile = (relPath) => fs.readFileSync(path.resolve(__dirname, relPath), 'utf8');

describe('page shell and pagination footer contracts', () => {
  it('core pages use shared page shell classes', () => {
    const pages = [
      'pages/Provider/index.js',
      'pages/Token/index.js',
      'pages/User/index.js',
      'pages/File/index.js',
      'pages/Log/index.js',
      'pages/Setting/index.js',
      'pages/Routes/index.js',
    ];

    pages.forEach((file) => {
      const source = readFile(file);
      expect(source).toContain('page-shell');
      expect(source).toContain('page-title');
    });
  });

  it('list tables use shared table footer class', () => {
    const tables = [
      'components/ProvidersTable.js',
      'components/AggTokensTable.js',
      'components/UsersTable.js',
      'components/FilesTable.js',
      'components/LogsTable.js',
    ];

    tables.forEach((file) => {
      const source = readFile(file);
      expect(source).toContain('table-footer');
    });
  });
});
