class Config {
  private static instance: Config;
  public readonly apiUrl: string;

  private constructor() {
    this.apiUrl = this.requireEnv('NEXT_PUBLIC_API_URL');
  }

  private requireEnv(name: string): string {
    const value = process.env[name];
    if (!value) {
      throw new Error(`Environment variable ${name} is not set`);
    }
    return value;
  }

  public static getInstance(): Config {
    if (!Config.instance) {
      Config.instance = new Config();
    }
    return Config.instance;
  }
}

export const config = Config.getInstance();