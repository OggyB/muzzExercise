package explore

import (
	"context"
	"strconv"
	"time"

	"github.com/oggyb/muzz-exercise/internal/app"
	svcErr "github.com/oggyb/muzz-exercise/internal/errors"
	pb "github.com/oggyb/muzz-exercise/internal/proto/explore"
	"github.com/oggyb/muzz-exercise/internal/repository"
)

// Service implements the Explore gRPC API.
// It contains the business logic on top of repository and cache layers.
// Each method corresponds to a gRPC endpoint defined in explore.proto.
type Service struct {
	appCtx       *app.AppContext
	decisionRepo *repository.DecisionRepository

	pb.UnimplementedExploreServiceServer
}

// NewExploreService creates a new Explore service with dependencies from AppContext.
// Dependencies include:
//   - DB connection (via DecisionRepository)
//   - RedisCache for counters from AppContext
func NewExploreService(appCtx *app.AppContext) *Service {
	return &Service{
		appCtx:       appCtx,
		decisionRepo: repository.NewDecisionRepository(appCtx.DB),
	}
}

// ListLikedYou returns all users who liked the given recipient.
//
// Behavior:
//   - Fetches likes for the given recipient via repository.GetLikers.
//   - Excludes users that the recipient explicitly passed.
//   - Supports cursor-based pagination with paginationToken.
//   - Returns actor_id + timestamp pairs.
//
// Example:
//
//	svc.ListLikedYou(ctx, &pb.ListLikedYouRequest{RecipientUserId: "42"})
func (s *Service) ListLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {

	s.appCtx.Logger.Debug("ListLikedYou called", "recipient", req.GetRecipientUserId(), "token", req.GetPaginationToken())

	recipientID, err := strconv.ParseUint(req.GetRecipientUserId(), 10, 64)
	if err != nil {
		s.appCtx.Logger.Error("Invalid recipient_user_id", "value", req.GetRecipientUserId(), "err", err)
		return nil, svcErr.InvalidArgument("recipient_user_id must be a valid uint64")
	}

	decisions, nextToken, err := s.decisionRepo.GetLikers(ctx, recipientID, req.PaginationToken, 5)
	if err != nil {
		s.appCtx.Logger.Error("GetLikers failed", "err", err)
		return nil, svcErr.Map(err)
	}

	resp := &pb.ListLikedYouResponse{}
	for _, d := range decisions {
		resp.Likers = append(resp.Likers, &pb.ListLikedYouResponse_Liker{
			ActorId:       strconv.FormatUint(d.ActorID, 10),
			UnixTimestamp: uint64(d.UpdatedAt.UnixMilli()),
		})
	}
	if nextToken != nil {
		resp.NextPaginationToken = nextToken
	}

	s.appCtx.Logger.Debug("ListLikedYou result", "liker_count", len(resp.Likers), "next_token", resp.GetNextPaginationToken())

	return resp, nil
}

// ListNewLikedYou returns all users who liked the recipient but have not been liked back.
//
// Behavior:
//   - Uses repository.GetNewLikers to exclude mutual likes.
//   - Excludes users the recipient explicitly passed.
//   - Returns actor_id + timestamp pairs.
//   - Supports cursor-based pagination.
//
// Example:
//
//	svc.ListNewLikedYou(ctx, &pb.ListLikedYouRequest{RecipientUserId: "42"})
func (s *Service) ListNewLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {
	s.appCtx.Logger.Debug("ListNewLikedYou called", "recipient", req.GetRecipientUserId())

	recipientID, err := strconv.ParseUint(req.GetRecipientUserId(), 10, 64)
	if err != nil {
		return nil, svcErr.InvalidArgument("recipient_user_id must be a valid uint64")
	}

	decisions, nextToken, err := s.decisionRepo.GetNewLikers(ctx, recipientID, req.PaginationToken, 5)
	if err != nil {
		return nil, svcErr.Map(err)
	}

	resp := &pb.ListLikedYouResponse{}
	for _, d := range decisions {
		resp.Likers = append(resp.Likers, &pb.ListLikedYouResponse_Liker{
			ActorId:       strconv.FormatUint(d.ActorID, 10),
			UnixTimestamp: uint64(d.UpdatedAt.UnixMilli()),
		})
	}
	if nextToken != nil {
		resp.NextPaginationToken = nextToken
	}

	return resp, nil
}

// CountLikedYou returns how many users liked the recipient.
// Cache-first strategy:
//  1. Attempts to read from Redis (likes:count:userID).
//  2. If cache miss or parse error, falls back to DB via repository.CountLikers.
//  3. On DB fetch, updates Redis with a 1h TTL.
//
// Example:
//
//	svc.CountLikedYou(ctx, &pb.CountLikedYouRequest{RecipientUserId: "42"})
func (s *Service) CountLikedYou(ctx context.Context, req *pb.CountLikedYouRequest) (*pb.CountLikedYouResponse, error) {
	s.appCtx.Logger.Debug("CountLikedYou called", "recipient", req.GetRecipientUserId())

	// parse recipient ID
	recipientID, err := strconv.ParseUint(req.GetRecipientUserId(), 10, 64)
	if err != nil {
		return nil, svcErr.InvalidArgument("recipient_user_id must be a valid uint64")
	}

	key := s.appCtx.RedisCache.KeyForLikeCount(recipientID)

	// try cache first
	if cached, _ := s.appCtx.RedisCache.Get(ctx, key); cached != "" {
		if n, err := strconv.ParseUint(cached, 10, 64); err == nil {
			// refresh TTL since this user is active
			_ = s.appCtx.RedisCache.Client.Expire(ctx, key, time.Hour).Err()
			return &pb.CountLikedYouResponse{Count: n}, nil
		}
	}

	// fallback: DB
	count, err := s.decisionRepo.CountLikers(ctx, recipientID)
	if err != nil {
		return nil, svcErr.Map(err)
	}

	// set + TTL refresh
	_ = s.appCtx.RedisCache.Set(ctx, key, strconv.FormatInt(count, 10), time.Hour)

	return &pb.CountLikedYouResponse{Count: uint64(count)}, nil
}

// PutDecision inserts or updates a decision and returns whether it resulted in a mutual like.
//
// Behavior:
//   - Validates actor and recipient IDs (must be different).
//   - Inserts/updates via repository.CreateOrUpdateDecision.
//   - Updates Redis like count (+1 or -1) with TTL refresh.
//   - If liked = true, checks for mutual like via repository.HasLiked.
//   - Returns whether mutual like exists.
//
// Example:
//
//	svc.PutDecision(ctx, &pb.PutDecisionRequest{ActorUserId: "1", RecipientUserId: "2", LikedRecipient: true})
func (s *Service) PutDecision(ctx context.Context, req *pb.PutDecisionRequest) (*pb.PutDecisionResponse, error) {
	s.appCtx.Logger.Debug(
		"PutDecision called",
		"actor", req.GetActorUserId(),
		"recipient", req.GetRecipientUserId(),
		"liked", req.GetLikedRecipient(),
	)
	actorID, err := strconv.ParseUint(req.GetActorUserId(), 10, 64)
	if err != nil {
		return nil, svcErr.InvalidArgument("actor_user_id must be a valid uint64")
	}
	recipientID, err := strconv.ParseUint(req.GetRecipientUserId(), 10, 64)
	if err != nil {
		return nil, svcErr.InvalidArgument("recipient_user_id must be a valid uint64")
	}

	if actorID == recipientID {
		return nil, svcErr.InvalidArgument("cannot decide on yourself")
	}

	// write/update decision
	if err := s.decisionRepo.CreateOrUpdateDecision(ctx, actorID, recipientID, req.GetLikedRecipient()); err != nil {
		return nil, svcErr.Map(err)
	}

	// update cache
	key := s.appCtx.RedisCache.KeyForLikeCount(recipientID)
	if req.GetLikedRecipient() {
		_, _ = s.appCtx.RedisCache.Incr(ctx, key) // like count +1
	} else {
		_, _ = s.appCtx.RedisCache.Decr(ctx, key) // like count -1
	}
	_ = s.appCtx.RedisCache.Client.Expire(ctx, key, time.Hour).Err() // refresh TTL

	// check if recipient also liked actor → mutual
	var mutual bool
	if req.GetLikedRecipient() {
		mutual, _ = s.decisionRepo.HasLiked(ctx, recipientID, actorID)
	}

	return &pb.PutDecisionResponse{MutualLikes: mutual}, nil
}
