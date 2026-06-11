import React, { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
  title: string;
  children: ReactNode;
  onReset?: () => void;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class WidgetErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(`Error caught in WidgetCard [${this.props.title}]:`, error, errorInfo);
  }

  private handleRetry = () => {
    this.setState({ hasError: false, error: null });
    if (this.props.onReset) {
      this.props.onReset();
    }
  };

  public render() {
    if (this.state.hasError) {
      return (
        <article className="flex flex-col justify-center items-center h-full p-6 bg-red-50/50 dark:bg-rose-950/20 border border-red-200 dark:border-rose-950 rounded-xl text-center">
          <span className="text-2xl mb-2" role="img" aria-label="Error Warning">🚨</span>
          <h4 className="text-sm font-bold text-red-900 dark:text-red-300">Widget Crashed</h4>
          <p className="text-xs text-red-700 dark:text-red-400 mt-1 max-w-[200px] truncate">
            {this.state.error?.name || 'Runtime Exception'}
          </p>
          <button
            onClick={this.handleRetry}
            className="mt-3 px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold rounded shadow transition"
          >
            Reset Widget
          </button>
        </article>
      );
    }

    return this.props.children;
  }
}
