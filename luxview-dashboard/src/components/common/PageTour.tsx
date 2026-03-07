import { useState, useEffect, useCallback, useMemo } from 'react';
import Joyride, { type Step, type CallBackProps, STATUS } from 'react-joyride';
import { useTranslation } from 'react-i18next';
import { useThemeStore } from '../../stores/theme.store';

export interface TourStep {
  target: string | HTMLElement;
  placement?: Step['placement'];
  title: string;
  content: string;
  disableBeacon?: boolean;
}

interface PageTourProps {
  tourId: string;
  steps: TourStep[];
  autoStart?: boolean;
}

const COMPLETED_PREFIX = 'lv_tour_';
const ONBOARDING_KEY = 'lv_onboarding_complete';

export function isOnboardingComplete(): boolean {
  return localStorage.getItem(ONBOARDING_KEY) === 'true';
}

export function isTourComplete(tourId: string): boolean {
  return localStorage.getItem(`${COMPLETED_PREFIX}${tourId}`) === 'true';
}

export function markTourComplete(tourId: string): void {
  localStorage.setItem(`${COMPLETED_PREFIX}${tourId}`, 'true');
}

export function markOnboardingComplete(): void {
  localStorage.setItem(ONBOARDING_KEY, 'true');
}

export function resetAllTours(): void {
  const keys = Object.keys(localStorage).filter(
    (k) => k.startsWith(COMPLETED_PREFIX) || k === ONBOARDING_KEY,
  );
  keys.forEach((k) => localStorage.removeItem(k));
}

export function skipAllTours(): void {
  markOnboardingComplete();
  ['dashboard', 'newApp', 'appDetail', 'resources', 'settings'].forEach(markTourComplete);
}

export function PageTour({ tourId, steps, autoStart = false }: PageTourProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { t } = useTranslation();
  const [run, setRun] = useState(false);

  useEffect(() => {
    if (autoStart && !isTourComplete(tourId)) {
      const timer = setTimeout(() => setRun(true), 500);
      return () => clearTimeout(timer);
    }
  }, [autoStart, tourId]);

  const translatedSteps: Step[] = useMemo(
    () =>
      steps.map((step) => ({
        ...step,
        title: t(step.title),
        content: t(step.content),
      })),
    [steps, t],
  );

  const handleCallback = useCallback(
    (data: CallBackProps) => {
      const { status } = data;
      if (status === STATUS.FINISHED || status === STATUS.SKIPPED) {
        setRun(false);
        markTourComplete(tourId);
        if (tourId === 'dashboard') {
          markOnboardingComplete();
        }
      }
    },
    [tourId],
  );

  if (translatedSteps.length === 0) return null;

  return (
    <Joyride
      steps={translatedSteps}
      run={run}
      continuous
      showSkipButton
      showProgress
      scrollToFirstStep
      callback={handleCallback}
      locale={{
        back: t('common.back'),
        close: t('common.close'),
        last: t('common.finish'),
        next: t('common.next'),
        skip: t('tour.skipTour'),
      }}
      styles={{
        options: {
          arrowColor: isDark ? '#27272a' : '#ffffff',
          backgroundColor: isDark ? '#27272a' : '#ffffff',
          primaryColor: '#f59e0b',
          textColor: isDark ? '#e4e4e7' : '#27272a',
          zIndex: 10000,
        },
        tooltip: {
          borderRadius: 16,
          padding: 20,
          border: isDark ? '1px solid #3f3f46' : '1px solid #e4e4e7',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
        },
        tooltipTitle: {
          fontSize: 15,
          fontWeight: 600,
        },
        tooltipContent: {
          fontSize: 13,
          lineHeight: 1.6,
          padding: '8px 0 0',
        },
        buttonNext: {
          borderRadius: 10,
          fontSize: 13,
          fontWeight: 500,
          padding: '8px 16px',
        },
        buttonBack: {
          borderRadius: 10,
          fontSize: 13,
          fontWeight: 500,
          color: isDark ? '#a1a1aa' : '#71717a',
        },
        buttonSkip: {
          fontSize: 12,
          color: isDark ? '#71717a' : '#a1a1aa',
        },
        spotlight: {
          borderRadius: 16,
        },
      }}
    />
  );
}
