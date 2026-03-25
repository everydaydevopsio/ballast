describe('ballast-typescript bin entrypoint', () => {
  test('invokes the CLI main function', () => {
    const main = jest.fn();
    jest.resetModules();
    jest.doMock('../dist/cli', () => ({ main }));

    jest.isolateModules(() => {
      require('../bin/ballast.js');
    });

    expect(main).toHaveBeenCalledTimes(1);
  });
});
