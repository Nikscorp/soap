import { Component, type ErrorInfo, type ReactNode } from 'react';
import { ErrorState } from './ErrorState';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  override state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('Unhandled error:', error, info);
  }

  override render(): ReactNode {
    if (this.state.error) {
      return (
        <div className="mx-auto mt-10 w-[90%] max-w-3xl rounded-md bg-white shadow-card">
          <ErrorState>
            Something went wrong: {this.state.error.message}. Try reloading the page.
          </ErrorState>
        </div>
      );
    }
    return this.props.children;
  }
}
