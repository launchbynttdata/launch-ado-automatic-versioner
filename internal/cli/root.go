package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/ado"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/config"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/branchmap"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/bump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/labels"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/domain/tagplan"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/logging"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/services/inferbump"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/services/prlabel"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/services/tagging"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/version"
)

const (
	envOrgURL     = "AAV_ORG_URL"
	envProject    = "AAV_PROJECT"
	envRepo       = "AAV_REPO"
	envToken      = "AAV_TOKEN"
	envLogLevel   = "AAV_LOG_LEVEL"
	envLabelPref  = "AAV_LABEL_PREFIX"
	envLabelMajor = "AAV_LABEL_MAJOR"
	envLabelMinor = "AAV_LABEL_MINOR"
	envLabelPatch = "AAV_LABEL_PATCH"

	envBranchMajor = "AAV_BRANCH_MAJOR_PREFIXES"
	envBranchMinor = "AAV_BRANCH_MINOR_PREFIXES"
	envBranchPatch = "AAV_BRANCH_PATCH_PREFIXES"

	envPRID         = "AAV_PR_ID"
	envSourceBranch = "AAV_SOURCE_BRANCH"

	envCommit = "AAV_COMMIT_SHA"
	envStrict = "AAV_STRICT"

	envTagMode         = "AAV_TAG_MODE"
	envBump            = "AAV_BUMP"
	envBaseVersion     = "AAV_BASE_VERSION"
	envTagMessage      = "AAV_TAG_MESSAGE"
	envTaggerName      = "AAV_TAGGER_NAME"
	envTaggerEmail     = "AAV_TAGGER_EMAIL"
	envTagPrefix       = "AAV_TAG_PREFIX"
	envUseFloatingTags = "AAV_USE_FLOATING_TAGS"
	requiredFlagFormat = "%s is required"
)

const (
	flagCommitSHA      = "commit-sha"
	flagTagMode        = "tag-mode"
	flagBump           = "bump"
	flagBaseVersion    = "base-version"
	flagTagMessage     = "tag-message"
	flagTaggerName     = "tagger-name"
	flagTaggerEmail    = "tagger-email"
	flagUseFloating    = "use-floating-tags"
	defaultTaggerName  = "aav"
	defaultTaggerEmail = "aav@example.com"
)

// Execute runs the CLI root command with the provided context.
func Execute(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return newRootCommand().ExecuteContext(ctx)
}

type rootFlagSet struct {
	orgURL      *stringFlag
	project     *stringFlag
	repo        *stringFlag
	token       *stringFlag
	logLevel    *stringFlag
	labelPref   *stringFlag
	labelMajor  *stringFlag
	labelMinor  *stringFlag
	labelPatch  *stringFlag
	branchMaj   *stringSliceFlag
	branchMin   *stringSliceFlag
	branchPatch *stringSliceFlag
}

type tagFlagSet struct {
	mode        *stringFlag
	bump        *stringFlag
	base        *stringFlag
	commit      *stringFlag
	message     *stringFlag
	taggerName  *stringFlag
	taggerEmail *stringFlag
	tagPrefix   *stringFlag
	useFloating *boolFlag
}

type runtimeConfig struct {
	resolver config.Resolver
	logger   *zap.Logger
	client   ado.Client
	branches branchmap.Resolver
	labels   labels.Resolver
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "aav",
		Short:         "ADO Automatic Versioner",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.Version = version.Version
	cmd.SetVersionTemplate("aav {{.Version}}\n")

	flags := bindRootFlags(cmd)
	cmd.AddCommand(
		newPRLabelCommand(flags),
		newInferCommand(flags),
		newTagCommand(flags),
		newVersionCommand(),
	)

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "aav %s\nbuild date: %s\n", version.Version, version.BuildDate); err != nil {
				return fmt.Errorf("writing version info: %w", err)
			}
			return nil
		},
	}
}

func bindRootFlags(cmd *cobra.Command) *rootFlagSet {
	defaults := branchmap.DefaultMapping()
	fs := cmd.PersistentFlags()
	return &rootFlagSet{
		orgURL:      bindStringFlag(fs, "org-url", "org-url", "", envOrgURL, "", "Azure DevOps organization URL"),
		project:     bindStringFlag(fs, "project", "project", "", envProject, "", "Azure DevOps project name"),
		repo:        bindStringFlag(fs, "repo", "repo", "", envRepo, "", "Azure DevOps repository name"),
		token:       bindSecretFlag(fs, "token", "token", "", envToken, "", "Azure DevOps personal access token or System.AccessToken"),
		logLevel:    bindStringFlag(fs, "log-level", "log-level", "", envLogLevel, logging.LevelTerse, "Log verbosity (terse or verbose)"),
		labelPref:   bindStringFlag(fs, "label-prefix", "label-prefix", "", envLabelPref, "semver-", "Optional prefix for semver labels"),
		labelMajor:  bindStringFlag(fs, "label-major", "label-major", "", envLabelMajor, "", "Override label name for major bumps"),
		labelMinor:  bindStringFlag(fs, "label-minor", "label-minor", "", envLabelMinor, "", "Override label name for minor bumps"),
		labelPatch:  bindStringFlag(fs, "label-patch", "label-patch", "", envLabelPatch, "", "Override label name for patch bumps"),
		branchMaj:   bindStringSliceFlag(fs, "branch-major-prefixes", "branch-major-prefix", "", envBranchMajor, defaults.MajorPrefixes, "Branch prefixes that imply a major bump"),
		branchMin:   bindStringSliceFlag(fs, "branch-minor-prefixes", "branch-minor-prefix", "", envBranchMinor, defaults.MinorPrefixes, "Branch prefixes that imply a minor bump"),
		branchPatch: bindStringSliceFlag(fs, "branch-patch-prefixes", "branch-patch-prefix", "", envBranchPatch, defaults.PatchPrefixes, "Branch prefixes that imply a patch bump"),
	}
}

func newPRLabelCommand(rootFlags *rootFlagSet) *cobra.Command {
	var prIDFlag *intFlag
	var branchFlag *stringFlag

	cmd := &cobra.Command{
		Use:   "pr-label",
		Short: "Ensure the expected semver label exists on a pull request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			runtime, cleanup, err := buildRuntime(ctx, rootFlags)
			if err != nil {
				return err
			}
			defer cleanup()

			prID, err := prIDFlag.Value(runtime.resolver)
			if err != nil {
				return err
			}
			if prID <= 0 {
				return fmt.Errorf("pr-id must be greater than zero")
			}

			branch := branchFlag.Value(runtime.resolver)
			if strings.TrimSpace(branch) == "" {
				return fmt.Errorf("source-branch is required")
			}

			service := prlabel.NewService(runtime.client, runtime.branches, runtime.labels)
			result, err := service.Apply(ctx, prlabel.Config{PRID: prID, Branch: branch})
			if err != nil {
				return err
			}

			log := runtime.logger.With(
				zap.Int("pr", prID),
				zap.String("branch", branch),
				zap.String("bump", result.Bump.String()),
				zap.Bool("branchMatched", result.BranchMatched),
				zap.String("matchedPrefix", result.MatchedPrefix),
			)

			switch result.Decision {
			case labels.DecisionAddExpected:
				log.Info("adding semver label", zap.String("label", result.ExpectedLabel))
			case labels.DecisionConflict:
				log.Warn("conflicting semver labels detected", zap.String("expected", result.ExpectedLabel), zap.Strings("existing", result.ExistingSemver))
			default:
				log.Info("expected semver label already present", zap.String("label", result.ExpectedLabel))
			}

			if result.LabelAdded {
				log.Info("semver label added", zap.String("label", result.ExpectedLabel))
			}

			return nil
		},
	}

	fs := cmd.Flags()
	prIDFlag = bindIntFlag(fs, "pr-id", "pr-id", "", envPRID, 0, "Pull request ID to label")
	branchFlag = bindStringFlag(fs, "source-branch", "source-branch", "", envSourceBranch, "", "Source branch name for the pull request")

	return cmd
}

func newInferCommand(rootFlags *rootFlagSet) *cobra.Command {
	var commitFlag *stringFlag
	var strictFlag *boolFlag

	cmd := &cobra.Command{
		Use:   "infer-bump",
		Short: "Infer bump intent from the merge commit's pull request labels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			runtime, cleanup, err := buildRuntime(ctx, rootFlags)
			if err != nil {
				return err
			}
			defer cleanup()

			commit := strings.TrimSpace(commitFlag.Value(runtime.resolver))
			if commit == "" {
				return fmt.Errorf(requiredFlagFormat, flagCommitSHA)
			}

			strict, err := strictFlag.Value(runtime.resolver)
			if err != nil {
				return err
			}

			return runInferCommand(cmd, ctx, runtime, commit, strict)
		},
	}

	fs := cmd.Flags()
	commitFlag = bindStringFlag(fs, flagCommitSHA, flagCommitSHA, "", envCommit, "", "Merge commit SHA to inspect")
	strictFlag = bindBoolFlag(fs, "strict", "strict", "", envStrict, false, "Fail when the merge commit cannot be mapped to a pull request")

	return cmd
}

func runInferCommand(cmd *cobra.Command, ctx context.Context, runtime runtimeConfig, commit string, strict bool) error {
	service := inferbump.NewService(runtime.client, runtime.labels)
	result, err := service.Resolve(ctx, inferbump.Config{CommitSHA: commit, Strict: strict})
	if err != nil {
		return err
	}

	log := runtime.logger.With(zap.String("commit", result.CommitSHA))
	if result.PRID > 0 {
		log = log.With(zap.Int("pr", result.PRID))
	}

	if result.Defaulted {
		log.Warn("default bump applied", zap.String("bump", result.Bump.String()), zap.String("reason", string(result.DefaultReason)))
	} else {
		log.Info("bump inferred", zap.String("bump", result.Bump.String()))
	}

	if len(result.SemverLabels) > 0 {
		log.Debug("semver labels considered", zap.Strings("labels", result.SemverLabels))
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), result.Bump.String()); err != nil {
		return fmt.Errorf("writing bump result: %w", err)
	}
	return nil
}

func newTagCommand(rootFlags *rootFlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-tag",
		Short: "Plan the next release or RC tag",
	}

	tagFlags := bindTagFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		runtime, cleanup, err := buildRuntime(ctx, rootFlags)
		if err != nil {
			return err
		}
		defer cleanup()

		createCfg, err := tagFlags.resolve(runtime.resolver)
		if err != nil {
			return err
		}

		tagPrefix := strings.TrimSpace(tagFlags.tagPrefix.Value(runtime.resolver))
		planner := tagplan.NewPlanner(tagPrefix)
		service := tagging.NewService(runtime.client, planner)
		result, err := service.PlanAndCreate(ctx, createCfg)
		if err != nil {
			return err
		}

		log := runtime.logger.With(
			zap.String("mode", string(result.Mode)),
			zap.String("tag", result.TagName),
			zap.String("releaseBase", result.ReleaseBase.String()),
			zap.String("baseSource", string(result.BaseSource)),
			zap.String("targetRelease", result.TargetRelease.String()),
			zap.String("commit", createCfg.CommitSHA),
			zap.String("tagger", createCfg.TaggerName),
		)
		if createCfg.Message != "" {
			log = log.With(zap.String("message", createCfg.Message))
		}
		if result.Mode == tagplan.ModeRC {
			log = log.With(zap.Int("rcNumber", result.RCNumber))
		}
		if tagPrefix != "" {
			log = log.With(zap.String("tagPrefix", tagPrefix))
		}
		log.Info("annotated tag created")

		if result.Mode == tagplan.ModeRelease {
			f := result.Floating
			switch {
			case f.Enabled:
				floatingLog := runtime.logger.With(zap.String("floatingTag", f.TagName))
				if f.DeletedExisting {
					floatingLog = floatingLog.With(zap.Bool("replaced", true))
				}
				if f.AutoDetected && !createCfg.UseFloatingTags {
					floatingLog = floatingLog.With(
						zap.Bool("autoEnabled", true),
						zap.Uint64("detectedMajor", f.AutoDetectedMajor),
					)
				}
				floatingLog.Info("floating tag updated")
			case createCfg.UseFloatingTags:
				runtime.logger.Warn("floating tag requested but not applied", zap.String("reason", "floating tags only apply to release mode"))
			case f.AutoDetected:
				runtime.logger.Info("floating tag usage detected", zap.Uint64("floatingMajor", f.AutoDetectedMajor))
			}
		}

		if _, err := fmt.Fprintln(cmd.OutOrStdout(), result.TagName); err != nil {
			return fmt.Errorf("writing tag result: %w", err)
		}
		return nil
	}

	return cmd
}

func bindTagFlags(cmd *cobra.Command) *tagFlagSet {
	fs := cmd.Flags()
	return &tagFlagSet{
		mode:        bindStringFlag(fs, flagTagMode, flagTagMode, "", envTagMode, "", "Tag mode to run (release or rc)"),
		bump:        bindStringFlag(fs, flagBump, flagBump, "", envBump, "", "Bump intent (major, minor, patch)"),
		base:        bindStringFlag(fs, flagBaseVersion, flagBaseVersion, "", envBaseVersion, "", "Optional base version to use when no releases exist"),
		commit:      bindStringFlag(fs, flagCommitSHA, flagCommitSHA, "", envCommit, "", "Commit SHA the tag should reference"),
		message:     bindStringFlag(fs, flagTagMessage, flagTagMessage, "", envTagMessage, "", "Message stored in the annotated tag"),
		taggerName:  bindStringFlag(fs, flagTaggerName, flagTaggerName, "", envTaggerName, defaultTaggerName, "Name recorded as the tagger"),
		taggerEmail: bindStringFlag(fs, flagTaggerEmail, flagTaggerEmail, "", envTaggerEmail, defaultTaggerEmail, "Email recorded as the tagger"),
		tagPrefix:   bindStringFlag(fs, "tag-prefix", "tag-prefix", "", envTagPrefix, "", "String prepended to computed tag names (e.g. 'v')"),
		useFloating: bindBoolFlag(fs, flagUseFloating, flagUseFloating, "", envUseFloatingTags, false, "Create/maintain floating major refs (v<major>)"),
	}
}

func (f *tagFlagSet) resolve(resolver config.Resolver) (tagging.CreateConfig, error) {
	modeValue := strings.TrimSpace(strings.ToLower(f.mode.Value(resolver)))
	if modeValue == "" {
		return tagging.CreateConfig{}, fmt.Errorf(requiredFlagFormat, flagTagMode)
	}
	mode, err := parseTagMode(modeValue)
	if err != nil {
		return tagging.CreateConfig{}, err
	}

	bumpValue := strings.TrimSpace(f.bump.Value(resolver))
	if bumpValue == "" {
		return tagging.CreateConfig{}, fmt.Errorf(requiredFlagFormat, flagBump)
	}
	bumpIntent, err := bump.Parse(bumpValue)
	if err != nil {
		return tagging.CreateConfig{}, err
	}

	baseVersion := strings.TrimSpace(f.base.Value(resolver))

	commit := strings.TrimSpace(f.commit.Value(resolver))
	if commit == "" {
		return tagging.CreateConfig{}, fmt.Errorf(requiredFlagFormat, flagCommitSHA)
	}

	taggerName := strings.TrimSpace(f.taggerName.Value(resolver))
	if taggerName == "" {
		return tagging.CreateConfig{}, fmt.Errorf(requiredFlagFormat, flagTaggerName)
	}

	taggerEmail := strings.TrimSpace(f.taggerEmail.Value(resolver))
	if taggerEmail == "" {
		return tagging.CreateConfig{}, fmt.Errorf(requiredFlagFormat, flagTaggerEmail)
	}

	message := strings.TrimSpace(f.message.Value(resolver))

	useFloating := false
	if f.useFloating != nil {
		value, err := f.useFloating.Value(resolver)
		if err != nil {
			return tagging.CreateConfig{}, err
		}
		useFloating = value
	}

	return tagging.CreateConfig{
		Config: tagging.Config{
			Mode:            mode,
			Bump:            bumpIntent,
			BaseVersion:     baseVersion,
			UseFloatingTags: useFloating,
		},
		CommitSHA:   commit,
		Message:     message,
		TaggerName:  taggerName,
		TaggerEmail: taggerEmail,
	}, nil
}

func buildRuntime(ctx context.Context, flags *rootFlagSet) (runtimeConfig, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	nopResolver := config.NewResolver(zap.NewNop())
	logLevel := flags.logLevel.Value(nopResolver)

	logger, err := logging.New(logLevel)
	if err != nil {
		return runtimeConfig{}, nil, fmt.Errorf("configuring logger: %w", err)
	}

	resolver := config.NewResolver(logger)
	_ = flags.logLevel.Value(resolver)

	orgURL := strings.TrimSpace(flags.orgURL.Value(resolver))
	if orgURL == "" {
		return runtimeConfig{}, nil, fmt.Errorf("org-url is required (set %s or --org-url)", envOrgURL)
	}

	project := strings.TrimSpace(flags.project.Value(resolver))
	if project == "" {
		return runtimeConfig{}, nil, fmt.Errorf("project is required (set %s or --project)", envProject)
	}

	repo := strings.TrimSpace(flags.repo.Value(resolver))
	if repo == "" {
		return runtimeConfig{}, nil, fmt.Errorf("repo is required (set %s or --repo)", envRepo)
	}

	token := strings.TrimSpace(flags.token.Value(resolver))
	if token == "" {
		return runtimeConfig{}, nil, fmt.Errorf("token is required (set %s or --token)", envToken)
	}

	labelResolver := labels.NewResolver(labels.Config{
		Prefix:     flags.labelPref.Value(resolver),
		MajorLabel: flags.labelMajor.Value(resolver),
		MinorLabel: flags.labelMinor.Value(resolver),
		PatchLabel: flags.labelPatch.Value(resolver),
	})

	branchResolver := branchmap.NewResolver(branchmap.Mapping{
		MajorPrefixes: flags.branchMaj.Value(resolver),
		MinorPrefixes: flags.branchMin.Value(resolver),
		PatchPrefixes: flags.branchPatch.Value(resolver),
	})

	client, err := ado.NewClient(ctx, ado.Config{
		OrganizationURL: orgURL,
		Project:         project,
		Repository:      repo,
		Token:           token,
	})
	if err != nil {
		return runtimeConfig{}, nil, err
	}

	cleanup := func() {
		_ = logger.Sync()
	}

	return runtimeConfig{
		resolver: resolver,
		logger:   logger,
		client:   client,
		branches: branchResolver,
		labels:   labelResolver,
	}, cleanup, nil
}

func parseTagMode(value string) (tagplan.Mode, error) {
	switch strings.ToLower(value) {
	case string(tagplan.ModeRelease):
		return tagplan.ModeRelease, nil
	case string(tagplan.ModeRC):
		return tagplan.ModeRC, nil
	default:
		return "", fmt.Errorf("invalid tag mode %q", value)
	}
}
