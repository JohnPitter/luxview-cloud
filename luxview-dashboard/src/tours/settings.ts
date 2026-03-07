import type { TourStep } from '../components/common/PageTour';

export const settingsTourSteps: TourStep[] = [
  {
    target: '[data-tour="theme-selector"]',
    title: 'tour.settings.theme.title',
    content: 'tour.settings.theme.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="language-selector"]',
    title: 'tour.settings.language.title',
    content: 'tour.settings.language.content',
  },
  {
    target: '[data-tour="replay-tours"]',
    title: 'tour.settings.tours.title',
    content: 'tour.settings.tours.content',
  },
];
