describe('ballast-typescript bin entrypoint', () => {
  afterEach(() => {
    jest.restoreAllMocks();
    jest.resetModules();
  });

  function loadBin(main: () => unknown): void {
    jest.doMock('../dist/cli', () => ({ main }), { virtual: true });

    jest.isolateModules(() => {
      require('../bin/ballast.js');
    });
  }

  test('invokes the CLI main function', () => {
    const main = jest.fn();
    loadBin(main);

    expect(main).toHaveBeenCalledTimes(1);
  });

  test('logs and exits non-zero when main throws synchronously', () => {
    const error = new Error('sync failure');
    const main = jest.fn(() => {
      throw error;
    });
    const consoleErrorSpy = jest
      .spyOn(console, 'error')
      .mockImplementation(() => {});
    const exitSpy = jest
      .spyOn(process, 'exit')
      .mockImplementation((() => undefined) as never);

    loadBin(main);

    expect(consoleErrorSpy).toHaveBeenCalledWith(error);
    expect(exitSpy).toHaveBeenCalledWith(1);
  });

  test('logs and exits non-zero when main returns a rejected promise', async () => {
    const error = new Error('async failure');
    const main = jest.fn(() => Promise.reject(error));
    const consoleErrorSpy = jest
      .spyOn(console, 'error')
      .mockImplementation(() => {});
    const exitSpy = jest
      .spyOn(process, 'exit')
      .mockImplementation((() => undefined) as never);

    loadBin(main);
    await new Promise<void>((resolve) => setImmediate(resolve));

    expect(consoleErrorSpy).toHaveBeenCalledWith(error);
    expect(exitSpy).toHaveBeenCalledWith(1);
  });
});
