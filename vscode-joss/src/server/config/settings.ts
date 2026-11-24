export interface JossSettings {
    enableJosSecurity: boolean;
    executablePath: string;
    format: {
        enable: boolean;
    };
    diagnostics: {
        compiler: {
            enable: boolean;
        };
    };
    inlayHints: {
        enable: boolean;
    };
    codeLens: {
        enable: boolean;
    };
}

export function getDefaultSettings(): JossSettings {
    return {
        enableJosSecurity: true,
        executablePath: 'joss',
        format: {
            enable: true
        },
        diagnostics: {
            compiler: {
                enable: true
            }
        },
        inlayHints: {
            enable: true
        },
        codeLens: {
            enable: true
        }
    };
}
